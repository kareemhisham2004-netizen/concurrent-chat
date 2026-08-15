package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

type User struct {
	Name  string
	Inbox chan string
	Done  chan struct{}
}

type Command struct {
	Action   string
	Username string
	Message  string
}

type Server struct {
	mu       sync.Mutex
	users    map[string]*User
	commands chan Command
	shutdown chan struct{}

	clientWG sync.WaitGroup
	serverWG sync.WaitGroup
}

var outputMu sync.Mutex

func printOutput(format string, args ...interface{}) {
	outputMu.Lock()
	defer outputMu.Unlock()

	fmt.Printf(format, args...)
}

func NewServer() *Server {
	return &Server{
		users:    make(map[string]*User),
		commands: make(chan Command),
		shutdown: make(chan struct{}),
	}
}

// Creates a new user and starts its goroutine.
func (s *Server) addUser(username string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reject duplicate usernames.
	if _, exists := s.users[username]; exists {
		return false
	}

	user := &User{
		Name:  username,
		Inbox: make(chan string, 32),
		Done:  make(chan struct{}),
	}

	s.users[username] = user

	s.clientWG.Add(1)
	go s.runClient(user)

	return true
}

// Removes a user.
func (s *Server) removeUser(username string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[username]

	if !exists {
		return false
	}

	delete(s.users, username)

	// Stop the user's goroutine.
	close(user.Done)

	return true
}

// Returns a snapshot of all connected users.
func (s *Server) getUsers() []*User {
	s.mu.Lock()
	defer s.mu.Unlock()

	users := make([]*User, 0, len(s.users))

	for _, user := range s.users {
		users = append(users, user)
	}

	return users
}

// Sends a message to every user except the sender.
func (s *Server) broadcast(message string, except string) {
	users := s.getUsers()

	for _, user := range users {
		if user.Name == except {
			continue
		}

		select {
		case user.Inbox <- message:
		case <-user.Done:
			// User has already left.
		case <-s.shutdown:
			return
		}
	}
}

// Each connected user has its own goroutine.
func (s *Server) runClient(user *User) {
	defer s.clientWG.Done()

	for {
		select {
		case message := <-user.Inbox:
			printOutput("\n[%s] %s\n> ", user.Name, message)

		case <-user.Done:
			return

		case <-s.shutdown:
			return
		}
	}
}

// Gracefully disconnect all users.
func (s *Server) closeAllUsers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for username, user := range s.users {
		close(user.Done)
		delete(s.users, username)
	}
}

// Server event loop.
func (s *Server) run() {
	defer s.serverWG.Done()

	for {
		select {

		case command := <-s.commands:

			switch command.Action {

			case "join":

				if command.Username == "" {
					printOutput(
						"\nUsername cannot be empty.\n> ",
					)
					continue
				}

				if !s.addUser(command.Username) {
					printOutput(
						"\nUsername '%s' already exists.\n> ",
						command.Username,
					)
					continue
				}

				printOutput(
					"\n%s has joined the chat.\n> ",
					command.Username,
				)

				s.broadcast(
					fmt.Sprintf(
						"User %s joined the chat.",
						command.Username,
					),
					command.Username,
				)

			case "users":

				users := s.getUsers()

				if len(users) == 0 {
					printOutput(
						"\nNo users connected.\n> ",
					)
					continue
				}

				printOutput("\n")

				for _, user := range users {
					printOutput(
						"  %s\n",
						user.Name,
					)
				}

				printOutput("> ")

			case "remove":

				if command.Username == "" {
					printOutput(
						"\nUsername cannot be empty.\n> ",
					)
					continue
				}

				if !s.removeUser(command.Username) {
					printOutput(
						"\nUser '%s' does not exist.\n> ",
						command.Username,
					)
					continue
				}

				printOutput(
					"\n%s has left the chat.\n> ",
					command.Username,
				)

				s.broadcast(
					fmt.Sprintf(
						"User %s left the chat.",
						command.Username,
					),
					"",
				)

			case "send":

				if command.Username == "" {
					printOutput(
						"\nNo user selected. Use: select <username>\n> ",
					)
					continue
				}

				if command.Message == "" {
					printOutput(
						"\nMessage cannot be empty.\n> ",
					)
					continue
				}

				s.mu.Lock()

				_, exists := s.users[command.Username]

				s.mu.Unlock()

				if !exists {
					printOutput(
						"\nUser '%s' does not exist.\n> ",
						command.Username,
					)
					continue
				}

				s.broadcast(
					fmt.Sprintf(
						"%s: %s",
						command.Username,
						command.Message,
					),
					command.Username,
				)
			}

		case <-s.shutdown:
			s.closeAllUsers()
			s.clientWG.Wait()
			return
		}
	}
}

func printHelp() {
	printOutput(`
Commands:

join <username>       Create a new chat user and connect them
users                 List all connected users
select <username>     Choose which user you're acting as
send <message>        Send a message as the currently selected user
remove <username>     Disconnect a user
who                   Show the currently selected user
help                  Show this help text
quit / exit           Gracefully shut down and exit

`)
}

func main() {

	server := NewServer()

	// Start the server goroutine.
	server.serverWG.Add(1)
	go server.run()

	// Channel for terminal commands.
	commandInput := make(chan Command)

	var inputWG sync.WaitGroup

	// Read terminal input concurrently.
	inputWG.Add(1)

	go func() {
		defer inputWG.Done()

		scanner := bufio.NewScanner(os.Stdin)

		for scanner.Scan() {

			line := strings.TrimSpace(scanner.Text())

			if line == "" {
				continue
			}

			parts := strings.Fields(line)

			command := Command{
				Action: parts[0],
			}

			switch parts[0] {

			case "join":

				if len(parts) >= 2 {
					command.Username = parts[1]
				}

			case "users":

			case "select":

				if len(parts) >= 2 {
					command.Username = parts[1]
				}

			case "send":

				if len(parts) >= 2 {
					command.Message = strings.TrimSpace(
						strings.TrimPrefix(line, "send"),
					)
				}

			case "remove":

				if len(parts) >= 2 {
					command.Username = parts[1]
				}

			case "who":

			case "help":

			case "quit", "exit":

				command.Action = "quit"

			default:

				command.Action = "unknown"
			}

			select {

			case commandInput <- command:

			case <-server.shutdown:
				return
			}
		}
	}()

	// Handle Ctrl+C.
	signalChan := make(chan os.Signal, 1)

	signal.Notify(
		signalChan,
		os.Interrupt,
		syscall.SIGTERM,
	)

	printOutput("Concurrent Chat System\n")
	printOutput("Type 'help' to see available commands.\n\n")
	printOutput("> ")

	selectedUser := ""

	shuttingDown := false

	for !shuttingDown {

		select {

		case command := <-commandInput:

			switch command.Action {

			case "join":

				server.commands <- command

			case "users":

				server.commands <- command

			case "select":

				server.mu.Lock()

				_, exists := server.users[command.Username]

				server.mu.Unlock()

				if !exists {
					printOutput(
						"\nUser '%s' does not exist.\n> ",
						command.Username,
					)
					continue
				}

				selectedUser = command.Username

				printOutput(
					"\nnow acting as %s\n> ",
					selectedUser,
				)

			case "send":

				command.Username = selectedUser

				server.commands <- command

			case "remove":

				server.commands <- command

				if command.Username == selectedUser {
					selectedUser = ""
				}

			case "who":

				if selectedUser == "" {
					printOutput(
						"\nNo user selected.\n> ",
					)
				} else {
					printOutput(
						"\nacting as: %s\n> ",
						selectedUser,
					)
				}

			case "help":

				printHelp()
				printOutput("> ")

			case "quit":

				shuttingDown = true

			case "unknown":

				printOutput(
					"\nUnknown command. Type 'help'.\n> ",
				)
			}

		case <-signalChan:

			printOutput("\n\nCtrl+C received.\n")
			shuttingDown = true
		}
	}

	printOutput("\nShutting down...\n")

	// Tell the server to shut down.
	close(server.shutdown)

	// Close stdin so the input goroutine can finish.
	_ = os.Stdin.Close()

	// Wait for server and client goroutines.
	server.serverWG.Wait()
	inputWG.Wait()

	printOutput("Goodbye!\n")
}
