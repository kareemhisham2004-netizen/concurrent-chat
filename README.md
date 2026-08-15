README.md
# Concurrent Chat System

A concurrent terminal-based chat system implemented in Go.

## Features

* Create new chat users
* List all connected users
* Select the user currently acting
* Send messages to other users
* Remove users from the chat
* Reject duplicate usernames
* Notify users when someone joins or leaves
* Show the currently selected user
* Graceful shutdown using `quit`, `exit`, or `Ctrl+C`

## Concurrency Concepts

This project demonstrates the following Go concurrency concepts:

* **Goroutines** — each connected user runs in a separate goroutine.
* **Channels** — used to send messages between the server and users.
* **Select** — used to handle multiple channel events concurrently.
* **sync.Mutex** — protects the shared users map from concurrent access.
* **sync.WaitGroup** — waits for goroutines to finish during shutdown.

## Requirements

* Go installed on the computer
* Go standard library only

 How to Run

Open a terminal inside the project folder and run:

```bash
go run .
```

## Commands

```text
join <username>       Create a new chat user and connect them
users                 List all connected users
select <username>     Choose which user you're acting as
send <message>        Send a message as the currently selected user
remove <username>     Disconnect a user
who                   Show the currently selected user
help                  Show this help text
quit / exit           Gracefully shut down and exit
```

## Example

```text
> join Kareem
Kareem has joined the chat.

> join Ahmed
Ahmed has joined the chat.

> users
  Kareem
  Ahmed

> select Kareem
now acting as Kareem

> send Hello Ahmed

> remove Ahmed
Ahmed has left the chat.

> quit

Shutting down...
Goodbye!
```

## Project Structure

```text
concurrent-chat/
├── main.go
├── go.mod
└── README.md
```
