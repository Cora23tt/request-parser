package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
)

type Request struct {
	Conn        net.Conn
	Method      string
	Path        string
	PathParam   string
	QueryParams url.Values
	Headers     map[string]string
	Body        string
}

func main() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("Ошибка запуска сервера:", err)
	}
	defer ln.Close()
	log.Println("Сервер запущен на :8080")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Ошибка подключения:", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	requestLine, _ := reader.ReadString('\n')
	log.Println("Request Line:", strings.TrimSpace(requestLine))

	parts := strings.Fields(requestLine)
	if len(parts) < 3 {
		log.Println("Неправильная строка запроса")
		return
	}
	method, pathWithQuery := parts[0], parts[1]

	u, _ := url.Parse(pathWithQuery)
	path := u.Path
	query := u.Query()
	log.Println("Метод:", method)
	log.Println("Путь:", path)
	log.Println("Query:", query.Encode())

	// получить параметр из path: /handler/{name}
	var name string
	if strings.HasPrefix(path, "/handler/") {
		name = strings.TrimPrefix(path, "/handler/")
	} else {
		name = "unknown"
	}
	log.Println("PathParam (handler):", name)

	// заголовки
	headers := make(map[string]string)
	for {
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		colonIndex := strings.Index(line, ":")
		if colonIndex != -1 {
			key := line[:colonIndex]
			value := strings.TrimSpace(line[colonIndex+1:])
			headers[key] = value
		}
	}
	log.Println("Заголовки:", headers)

	// тело
	body := make([]byte, 0)
	if contentLengthStr, ok := headers["Content-Length"]; ok {
		var contentLength int
		fmt.Sscanf(contentLengthStr, "%d", &contentLength)

		body = make([]byte, contentLength)
		_, err := io.ReadFull(reader, body)
		if err != nil {
			log.Println("Ошибка при чтении тела:", err)
			httpError(conn, "Ошибка при чтении тела запроса")
			return
		}
		log.Println("Тело запроса:", string(body))
	} else {
		log.Println("Content-Length не найден")
	}

	req := Request{
		Conn:        conn,
		Method:      method,
		Path:        path,
		PathParam:   name,
		QueryParams: query,
		Headers:     headers,
		Body:        string(body),
	}

	respondHTML(req)
}

func respondHTML(req Request) {
	tmpl, err := template.ParseFiles("static/layout_handler.html")
	if err != nil {
		log.Println("Ошибка шаблона:", err)
		httpError(req.Conn, "Template error")
		return
	}

	queryDump := req.QueryParams.Encode()
	var headerDump strings.Builder
	for k, v := range req.Headers {
		headerDump.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}

	data := map[string]string{
		"handler": req.PathParam,
		"query":   queryDump,
		"headers": headerDump.String(),
		"body":    req.Body,
	}

	var resp bytes.Buffer
	resp.WriteString("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n\r\n")
	tmpl.Execute(&resp, data)
	req.Conn.Write(resp.Bytes())
	log.Println("Ответ отправлен клиенту")
}

func httpError(conn net.Conn, msg string) {
	conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n\r\n" + msg))
}
