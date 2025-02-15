# Use the official Golang image matching your go.mod version
FROM golang:1.22-alpine

# Set the working directory
WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o backend cmd/main.go

# Expose the application port
EXPOSE 8080

# Run the application
CMD ["./backend"]