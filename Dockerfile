# Use Go base image
FROM golang:latest

# Set working directory
WORKDIR /app

# Copy and install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o main .

# Expose the port
EXPOSE 8080

# Run the application
CMD ["./main"]