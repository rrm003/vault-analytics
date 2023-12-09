package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"cloud.google.com/go/pubsub"
	firebaseAdmin "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/appcheck"
	"firebase.google.com/go/v4/auth"
	"github.com/go-pg/pg/v10"

	"github.com/gorilla/mux"
	"google.golang.org/api/option"
)

var (
	appCheck   *appcheck.Client
	authClient *auth.Client
)

type TopicSubscriber struct {
	Topic        string
	Subscription string
}

type Event struct {
	UserID    string `json:"user_id"`
	Action    string `json:"action"`
	IP        string `json:"ip"`
	Browser   string `json:"browser"`
	Timestamp string `json:"timestamp"`
}

type AppSvc struct {
	Ctx    context.Context
	DB     *pg.DB
	Logger *log.Logger
}

func requireAppCheck(handler func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	wrappedHandler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Auth key", r.Header[http.CanonicalHeaderKey("X-Firebase-AppCheck")])
		appCheckToken, ok := r.Header[http.CanonicalHeaderKey("X-Firebase-AppCheck")]
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized."))
			return
		}

		token, err := authClient.VerifyIDToken(r.Context(), appCheckToken[0])
		if err != nil {
			fmt.Println("failed to get the token", err)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized."))
			return
		}

		fmt.Printf("token iD %+v\n", token)

		ctx := context.WithValue(r.Context(), "userid", token.UID)

		handler(w, r.WithContext(ctx))
	}

	return wrappedHandler
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff here
		log.Println(r.RequestURI)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set the CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Continue processing the request
		next.ServeHTTP(w, r)
	})
}

func pullMsgs(app *AppSvc, w io.Writer, projectID, subID string) error {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return fmt.Errorf("pubsub.NewClient: %w", err)
	}
	defer client.Close()

	sub := client.Subscription(subID)

	var received int32
	for {
		// Receive a single message at a time without a timeout
		err := sub.Receive(ctx, func(_ context.Context, msg *pubsub.Message) {
			fmt.Fprintf(w, "Got message from %s: %q\n", subID, string(msg.Data))

			e := Event{}
			err = json.Unmarshal(msg.Data, &e)
			if err != nil {
				fmt.Printf("filed to read the event details %v \n", err)
			} else {
				err = app.storeEvent(e)
				if err != nil {
					fmt.Printf("failed to send the mail %v", err)
				}
			}

			atomic.AddInt32(&received, 1)
			msg.Ack()
		})
		if err != context.Canceled {
			// If the context was canceled (e.g., program exit), break the loop
			break
		} else if err != nil {
			return fmt.Errorf("sub.Receive: %w", err)
		}
	}
	fmt.Fprintf(w, "Received %d messages from %s\n", received, subID)

	return nil
}

func main() {
	os.Setenv("DB_HOST", "34.70.210.195")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("DB_USER", "postgres")
	os.Setenv("DB_PASSWORD", "loop@007")
	os.Setenv("DB_NAME", "vault-db")
	os.Setenv("PROJECT_ID", "vault-svc")

	r := mux.NewRouter()
	r.Use(enableCORS)

	ctx := context.Background()

	app := &AppSvc{}
	app.Ctx = ctx

	//valut-svc-firebase-adminsdk4
	opt := option.WithCredentialsFile("./valut-svc-firebase-adminsdk4.json")

	admin, err := firebaseAdmin.NewApp(ctx, nil, opt)
	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
		return
	}

	appCheck, err = admin.AppCheck(ctx)
	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
		return
	}

	// Create a Firebase auth client instance
	authClient, err = admin.Auth(ctx)
	if err != nil {
		log.Fatalf("Failed to create Firebase auth client: %v", err)
		return
	}

	pgdb, err := StartDB()
	if err != nil {
		log.Printf("error starting the database %v", err)
		return
	}

	app.DB = pgdb

	projectID := "valut-svc" // Replace with your Google Cloud project ID

	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		return
	}

	fileName := "valut-svc-firebase-adminsdk4.json"
	filePath := filepath.Join(currentDir, fileName)

	// Set the environment variable
	err = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filePath)
	if err != nil {
		fmt.Println("Error setting environment variable:", err)
		return
	}

	// Create a list of topics and their subscribers
	topicSubscribers := []TopicSubscriber{
		{Topic: "topic-audit", Subscription: "topic-audit-sub"},
		// {Topic: "topic2", Subscription: "topic2-sub"},
		// Add more topics and subscribers as needed
	}

	// Use a WaitGroup to wait for all subscribers to finish
	var wg sync.WaitGroup

	for _, ts := range topicSubscribers {
		wg.Add(1)

		go func(ts TopicSubscriber) {
			defer wg.Done()

			fmt.Printf("Starting subscriber for topic: %s, subscription: %s\n", ts.Topic, ts.Subscription)

			if err := pullMsgs(app, os.Stdout, projectID, ts.Subscription); err != nil {
				fmt.Printf("Error receiving messages for topic %s: %v\n", ts.Topic, err)
			}
		}(ts)
	}

	fmt.Println("Press Enter to stop receiving messages...")
	ready := make(chan struct{})

	go func() {
		// Wait for user input to stop the program
		bufio.NewReader(os.Stdin).ReadString('\n')
		close(ready)
	}()

	r.Use(loggingMiddleware)

	go func() {
		// Define the endpoint for getting paginated items
		r.HandleFunc("/logs", requireAppCheck(app.getItemsHandler)).Methods(http.MethodGet, http.MethodOptions)

		// Start the HTTP server on port 8080
		log.Fatal(http.ListenAndServe(":8082", r))
	}()

	// Wait for all subscribers to finish
	wg.Wait()

	// Close the ready channel to signal that all subscribers have completed
	<-ready
}
