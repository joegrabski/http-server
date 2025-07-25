package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

type Verb string

const (
	GET  Verb = "GET"
	POST Verb = "POST"
)

type RequestLine struct {
	Verb    Verb
	Path    string
	Version string
}

type Request struct {
	RequestLine RequestLine
	Headers     map[string]string
	Body        string
}

type Status struct {
	Code   int
	Status string
}

type Response struct {
	Status  Status
	Headers map[string]string
	Body    string
}

type Endpoint func(request Request) Response

type Router struct {
	routes map[string]Endpoint
}

func parseRequest(buf []byte) Request {
	request := string(buf)
	fmt.Printf("\nReceived request: \n%s", request)

	// Split the request into lines
	lines := strings.Split(request, "\r\n")

	status := lines[0]

	// Parse headers
	headers := make(map[string]string)
	i := 1

	// Iterate over the lines to parse headers
	for i < len(lines) && lines[i] != "" {
		headerParts := strings.SplitN(lines[i], ":", 2) // Split only on the first colon
		if len(headerParts) == 2 {
			key := strings.ToLower(strings.TrimSpace(headerParts[0]))
			value := strings.TrimSpace(headerParts[1])
			headers[key] = value
		}
		i++
	}

	// The body starts after the headers and the blank line
	bodyLines := lines[i:]
	body := strings.Join(bodyLines, "\r\n")

	return Request{
		RequestLine: parseRequestLine(status),
		Headers:     headers,
		Body:        body,
	}
}

func parseRequestLine(status string) RequestLine {
	requestLineParts := strings.Split(status, " ")
	verb := Verb(requestLineParts[0])
	path := requestLineParts[1]
	version := requestLineParts[2]
	return RequestLine{
		Verb:    verb,
		Path:    path,
		Version: version,
	}
}

func writeResponseToBytes(response Response) []byte {
	result := fmt.Sprintf("HTTP/1.1 %d %s\r\n", response.Status.Code, response.Status.Status)

	// Append headers
	for key, value := range response.Headers {
		result += fmt.Sprintf("%s: %s\r\n", key, value)
	}

	// Add the blank line between headers and body
	result += "\r\n"

	result += response.Body

	fmt.Printf("\nResponding with: \n%s", result)

	return []byte(result)
}

func HTTPRouter() *Router {
	return &Router{routes: make(map[string]Endpoint)}
}

// Register adds a new endpoint to the router.
func (r *Router) Register(path string, verb Verb, handler Endpoint) {
	key := path + string(verb)
	r.routes[key] = handler
}

func (r *Router) Handle(request Request) Response {
	// Handle looks up an endpoint by path and invokes it.
	key := request.RequestLine.Path + string(request.RequestLine.Verb)
	if handler, exists := r.routes[key]; exists {
		return handler(request)
	}
	// Return a 404 response if the route is not found.
	return Response{
		Status:  Status{Code: 404, Status: "Not Found"},
		Headers: map[string]string{"Content-Type": "text/plain"},
		Body:    "404 - Not Found",
	}
}

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	router := HTTPRouter()

	router.Register("/", GET, func(request Request) Response {
		data, err := os.ReadFile("./temp/index.html")
		if err != nil {
			return Response{
				Status: Status{Code: 500, Status: "Internal Server Error"},
			}
		}
		return Response{
			Status: Status{Code: 200, Status: "OK"},
			Headers: map[string]string{
				"Content-Type":   "text/html",
				"Content-Length": fmt.Sprintf("%d", len(data)),
			},
			Body: string(data),
		}
	})

	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			c.Close()
		}

		buf := make([]byte, 2048)
		_, err = c.Read(buf)
		if err != nil {
			fmt.Println("Error reading connection: ", err.Error())
			c.Close()
		}

		request := parseRequest(buf)
		response := router.Handle(request)

		c.Write(writeResponseToBytes(response))
		c.Close()
	}
}
