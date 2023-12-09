# Use the official Golang base image
FROM golang:1.19

# Set the working directory inside the container
WORKDIR /app

# Define environment variables
# ENV SENDGRID_API_KEY=XXX

# Copy the source code to the container
COPY . .

# Build the Go application
RUN go build -o main .

# Expose the port your application will run on
EXPOSE 8082

# Define the command to run your application
CMD ["./main"]
