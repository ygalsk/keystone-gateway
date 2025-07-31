# Claude Configuration for Go Project

## Bash Commands
- go mod init: Initialize module
- go mod tidy: Clean up dependencies
- go build: Build the project
- go test ./...: Run all tests
- go fmt ./...: Format code
- go vet ./...: Run static analysis

## Code Style
- Follow Go conventions and idioms
- Use gofmt for formatting
- Prefer composition over inheritance
- Handle errors explicitly

## Project Structure
- cmd/ for main applications
- pkg/ for public packages
- internal/ for private packages
- Keep packages small and focused

## Testing
- Write table-driven tests
- Use testify for assertions
- Mock external dependencies
- Benchmark critical paths