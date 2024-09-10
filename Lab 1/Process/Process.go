package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/fatih/color"
)

// Definition of states using iota to simulate enum
const (
	RELEASED = iota // 0: The process is not attempting to access the CS
	WANTED          // 1: The process wants to access the CS
	HELD            // 2: The process is in the CS
)

// Constant for the SharedResource port
const sharedResourcePort = ":10001"

// Struct definitions

// Message that will be sent between processes
type Message struct {
	ProcessId int    // Uppercase to export fields for visibility in JSON
	Text      string // Message text
	Clock     int    // Logical clock of the message
}

// Message that will be sent to the shared resource
type MessageToSharedResource struct {
	ProcessId int    // Process ID that sent the message
	Text      string // Text of the message
	Clock     int    // Logical clock of the message
}

// Global variables for the process

// Current process
var processId int    // ID of the current process
var clock int        // Logical clock of the current process
var requestClock int // Clock of the request, updated when entering the WANTED state, used to compare with received requests
var state int        // State of the process: RELEASED, WANTED, or HELD (Ricart-Agrawala)

// Other important variables
var replyMap map[int]bool  // Map to track which processes have already replied
var requestQueue []Message // Queue of requests received but not yet processed
var isReleasing bool       // Flag to indicate if the process is currently releasing the CS

var myPort string // Server port for this process
// var CliConn []*net.UDPConn // Old: Vector with connections to other servers
var CliConn map[int]*net.UDPConn    // New: Map of connections to other processes (maps process ID to UDP connection)
var sharedResourceConn *net.UDPConn // Connection to the SharedResource (critical section)

// Variables for other processes
var nServers int          // Number of other processes
var ServConn *net.UDPConn // Server connection for receiving messages from other processes

// Routine to "listen" to stdin (keyboard input)
func readInput(ch chan string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		text, _, _ := reader.ReadLine()
		ch <- string(text)
	}
}

// Error handling function
func CheckError(err error) {
	if err != nil {
		color.Set(color.FgRed)
		fmt.Println("Error:", err)
		color.Unset()
		os.Exit(0)
	}
}

// Function to print error messages
func PrintError(err error) {
	if err != nil {
		color.Set(color.FgRed)
		fmt.Println("Error:", err)
		color.Unset()
	}
}

// Function to print the current state with colors and formatting
func printState() {
	var stateStr string
	var stateColor *color.Color

	switch state {
	case RELEASED:
		stateStr = "RELEASED"
		stateColor = color.New(color.FgGreen).Add(color.Bold)
	case WANTED:
		stateStr = " WANTED "
		stateColor = color.New(color.FgYellow).Add(color.Bold)
	case HELD:
		stateStr = "  HELD  "
		stateColor = color.New(color.FgRed).Add(color.Bold)
	}

	// Format and print the state centered
	color.Set(color.FgHiWhite)
	fmt.Print("\n+------------------+\n")
	fmt.Printf("|     ")
	stateColor.Printf("%-8s", stateStr) // Display the state with color and centered
	color.Set(color.FgHiWhite)
	fmt.Print("     |\n")
	fmt.Print("+------------------+\n")
	color.Unset() // Reset color to default
}

// Function responsible for receiving messages from other processes
func doServerJob() {
	buf := make([]byte, 1024)
	for {
		// Read a message from the UDP connection
		n, _, err := ServConn.ReadFromUDP(buf)
		PrintError(err)

		// Deserialize the received message
		var receivedMsg Message
		err = json.Unmarshal(buf[:n], &receivedMsg)
		PrintError(err)

		if receivedMsg.Text == "request" {
			// Handle the request message
			handleRequest(receivedMsg)
		} else if receivedMsg.Text == "reply" {
			// Handle the reply message
			handleReply(receivedMsg)
		}
	}
}

// Handle the received request message
func handleRequest(receivedMsg Message) {
	// Update the logical clock based on the received message
	clock = max(clock, receivedMsg.Clock) + 1

	// Display the received message
	color.Set(color.FgHiWhite)
	fmt.Printf("Received '%s' from process %d with clock %d | Current clock: %d\n", receivedMsg.Text, receivedMsg.ProcessId, receivedMsg.Clock, clock)
	color.Unset()

	// Decide whether to queue the request or reply immediately
	if state == HELD || (state == WANTED && (requestClock < receivedMsg.Clock || (requestClock == receivedMsg.Clock && processId < receivedMsg.ProcessId))) {
		// Queue the request, do not reply immediately
		color.Set(color.FgHiWhite)
		fmt.Printf("Queuing request from process %d (clock = %d)\n", receivedMsg.ProcessId, receivedMsg.Clock)
		color.Unset()
		requestQueue = append(requestQueue, receivedMsg)
	} else {
		// Send an immediate reply
		color.Set(color.FgHiWhite)
		fmt.Printf("Sending immediate reply to process %d\n", receivedMsg.ProcessId)
		color.Unset()
		go doClientJob(receivedMsg.ProcessId, "reply", clock) // Reply without incrementing the clock
	}
}

// Handle the received reply message
func handleReply(receivedMsg Message) {
	if state == WANTED {
		if !replyMap[receivedMsg.ProcessId] {
			// Mark that a reply was received from this process
			replyMap[receivedMsg.ProcessId] = true
			color.Set(color.FgHiWhite)
			fmt.Printf("Reply received from process %d | Total replies: %d\n", receivedMsg.ProcessId, len(replyMap))
			color.Unset()
		} else {
			// Ignore duplicate replies
			color.Set(color.FgHiWhite)
			fmt.Printf("Duplicate reply received from process %d\n", receivedMsg.ProcessId)
			color.Unset()
		}
	}
}

// Function to send messages to other processes
func doClientJob(otherProcessId int, text string, clock int) {
	// Create the message to be sent (e.g., "request" or "reply")
	msg := Message{ProcessId: processId, Text: text, Clock: clock}

	// Serialize the message to JSON
	jsonMsg, err := json.Marshal(msg)
	PrintError(err)

	// Send the message to the corresponding process using the map of connections
	conn := CliConn[otherProcessId]
	_, err = conn.Write(jsonMsg)
	PrintError(err)
}

// Handle keyboard input (stdin)
func handleKeyboardInput(input string) {
	if input == "x" {
		// If the process is already in CS or waiting, ignore "x"
		if state == WANTED || state == HELD {
			color.Set(color.FgHiWhite)
			fmt.Println("Ignored 'x', process is already waiting or in critical section")
			color.Unset()
		} else {
			if isReleasing {
				color.Set(color.FgHiWhite)
				fmt.Println("Ignored 'x', process is currently releasing the critical section")
				color.Unset()
			} else {
				// If 'x' is received, request access to the critical section
				requestCriticalSection()
			}
		}
	} else if id, err := strconv.Atoi(input); err == nil && id == processId {
		// If the process ID is received, increment the logical clock
		clock++
		color.Set(color.FgHiWhite)
		fmt.Printf("Internal event! | Clock: %d\n", clock)
		color.Unset()
	} else {
		// Invalid input
		color.Set(color.FgRed)
		fmt.Printf("Invalid input '%s'. Enter 'x' or the process ID (%d).\n", input, processId)
		color.Unset()
	}
}

// Request access to the critical section
func requestCriticalSection() {
	clock++
	requestClock = clock // Store the current clock for requests
	state = WANTED
	printState() // Print the state after the change

	color.Set(color.FgHiWhite)
	fmt.Println("Requesting access to the critical section (sending requests)... | Clock:", requestClock)
	color.Unset()

	// Send requests to all other processes
	for otherProcessId := 1; otherProcessId <= nServers+1; otherProcessId++ {
		if otherProcessId != processId { // Don't send a request to myself
			go doClientJob(otherProcessId, "request", requestClock)
		}
	}

	// Wait until N-1 replies are received
	for len(replyMap) < nServers {
		time.Sleep(2000 * time.Millisecond) // Wait a short interval before checking again
	}

	// Simulate entering the critical section (received replies from all processes)
	enterCriticalSection()
}

// Enter the critical section
func enterCriticalSection() {
	state = HELD
	printState() // Print the state after the change

	// Get the local port of the sharedResourceConn connection
	localAddr := sharedResourceConn.LocalAddr().(*net.UDPAddr) // Convert to *net.UDPAddr
	color.Set(color.FgHiWhite)
	fmt.Printf("Entering critical section through port %d\n", localAddr.Port) // Show the local port used
	color.Unset()

	// Create the message to be sent to the SharedResource
	msg := MessageToSharedResource{
		ProcessId: processId,
		Text:      "Hello, CS!",
		Clock:     clock,
	}

	// Serialize the message to JSON
	jsonMsg, err := json.Marshal(msg)
	CheckError(err)

	// Send the message to the SharedResource using the global connection `sharedResourceConn`
	_, err = sharedResourceConn.Write(jsonMsg)
	CheckError(err)

	// Simulate using the CS (sleep for 10 seconds)
	time.Sleep(10 * time.Second)

	// After using the CS, release it
	releaseCriticalSection()
}

// Release the critical section
func releaseCriticalSection() {
	// The channel indicates that releaseCriticalSection has started
	isReleasing = true

	state = RELEASED
	printState() // Print the state after the change
	color.Set(color.FgHiWhite)
	fmt.Println("Releasing the critical section...") // Show release message
	color.Unset()

	// Send replies to all processes in the requestQueue
	for _, request := range requestQueue {
		color.Set(color.FgHiWhite)
		fmt.Printf("Sending reply to process %d\n", request.ProcessId)
		color.Unset()

		// Send the reply message to the queued process
		go doClientJob(request.ProcessId, "reply", clock)
	}
	requestQueue = []Message{} // Clear the request queue

	replyMap = make(map[int]bool) // Reset the reply map

	// When finished, release the channel
	isReleasing = false
}

// Initialize the connections with other processes and the shared resource
func initConnections() {
	myPort = os.Args[1+processId]        // My server port
	nServers = len(os.Args) - 3          // Calculate the number of servers (other processes)
	CliConn = make(map[int]*net.UDPConn) // Initialize the map of connections

	// Set up the server to receive messages
	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort)
	CheckError(err)
	ServConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)

	// Initialize the connection to the SharedResource (critical section)
	sharedResourceAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+sharedResourcePort)
	CheckError(err)
	sharedResourceConn, err = net.DialUDP("udp", nil, sharedResourceAddr) // Create a global connection to the SharedResource
	CheckError(err)

	// Initialize connections to each server for other processes, storing in CliConn map
	for s := 0; s < nServers+1; s++ {
		if s == processId-1 { // Skip my own port
			continue
		}

		otherProcessId := s + 1

		// Resolve the server address
		ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+os.Args[2+s])
		CheckError(err)

		// Establish the UDP connection and store it in the map
		Conn, err := net.DialUDP("udp", nil, ServerAddr)
		CliConn[otherProcessId] = Conn
		CheckError(err)
	}
}

// Main function
func main() {
	// Expected parameters: process ID, server ports
	processId, _ = strconv.Atoi(os.Args[1])

	// Check if the number of arguments is sufficient
	if len(os.Args) < 2+processId {
		color.Set(color.FgRed)
		fmt.Println("Error: Insufficient arguments. Check the number of processes and ports.")
		color.Unset()
		os.Exit(1)
	}

	// Check if any of the ports are the SharedResource port (10001)
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == sharedResourcePort {
			color.Set(color.FgRed)
			fmt.Printf("Error: Port %s is reserved for the SharedResource. Choose another port.\n", sharedResourcePort)
			color.Unset()
			os.Exit(1)
		}
	}

	// Initialize the process clock
	clock = 0
	state = RELEASED // Initial state is RELEASED, meaning the process is neither in CS nor waiting
	printState()     // Print the state after the initialization

	// Initialize the reply map and the isReleasing flag
	replyMap = make(map[int]bool)
	isReleasing = false

	// Initialize the connections with other processes
	initConnections()

	// Defer connection closure to ensure they are closed when the program terminates
	defer ServConn.Close()
	defer sharedResourceConn.Close()
	for _, conn := range CliConn {
		defer conn.Close() // Close each connection stored in the map
	}

	// Set up a channel to handle keyboard input
	ch := make(chan string) // Channel to store keyboard input
	go readInput(ch)        // Call routine to "listen" to the keyboard (stdin)

	go doServerJob() // Start server job to receive messages
	for {
		// Check for input from the terminal (stdin) in a non-blocking manner
		select {
		case input, valid := <-ch: // Also handle the case where the channel is closed (valid)
			if valid {
				handleKeyboardInput(input) // Function to handle keyboard input
			} else {
				// Channel closed
				color.Set(color.FgRed)
				fmt.Println("Closed Keyboard channel!")
				color.Unset()
			}
		default:
			// Do nothing
			time.Sleep(time.Second * 1)
		}

		// Wait a bit
		time.Sleep(time.Second * 1)
	}
}
