package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

type Message struct {
	Code int    // Devemos usar letra maiúscula para definir os campos da struct.
	Text string // Assim eles serão exportados, ou seja, Vistos por outras packages (incluindo o json).
}

// Variáveis globais interessantes para o procseso
var myPort string          // porta do meu servidor
var nServers int           // qtde de outros processos
var CliConn []*net.UDPConn // vetor com conexões para os servidores
// dor outros processos
var ServConn *net.UDPConn // conexão do meu servidor (onde recebo mensagens dos outros processos)

func readInput(ch chan string) {
	// Rotina que "escuta" o stdin
	reader := bufio.NewReader(os.Stdin)
	for {
		text, _, _ := reader.ReadLine()
		ch <- string(text)
	}
}

func CheckError(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(0)
	}
}

func PrintError(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
	}
}

func doServerJob() {
	buf := make([]byte, 1024)
	for {
		// Ler (uma vez somente) da conexão UDP a mensagem
		n, addr, err := ServConn.ReadFromUDP(buf)
		PrintError(err)
		// Escrever na tela a msg recebida (indicando o endereço de quem enviou)
		fmt.Println("Received ", string(buf[0:n]), " from ", addr)
	}
}

func doClientJob(otherProcess int, i int) {
	msg := Message{123, "Teste"}      // A mensagem será fixa. Sempre com Code 123 e Text "teste"
	jsonMsg, err := json.Marshal(msg) // Agora json fará parte dos imports
	PrintError(err)

	_, err = CliConn[otherProcess].Write(jsonMsg)
	PrintError(err)
}

func initConnections() {
	myPort = os.Args[1]
	nServers = len(os.Args) - 2 // Esse 2 tira o nome ("Process") e tira a primeira porta (que é a minha). As demais portas são dos outros processos
	CliConn = make([]*net.UDPConn, nServers)

	/* Outros códigos para deixar ok a conexão do meu servidor (one recebo msgs). O processo já deve ficar habilitado a receber msgs. */

	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort)
	CheckError(err)
	ServConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)

	/* Outros códigos para deixar ok a minha conexão com cada servidor dos outros processos. Colocar tais conexões no vetor CliConn. */
	for s := 0; s < nServers; s++ {
		ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+os.Args[2+s])
		CheckError(err)

		/* Aqui não foi definido o endereço do cliente. Usando nil, o próprio sistema escolhe. */
		Conn, err := net.DialUDP("udp", nil, ServerAddr)
		CliConn[s] = Conn
		CheckError(err)
	}
}

func main() {
	initConnections()

	/* O fechamento de conexões deve ficar aqui, assim só fecha conexão quando a main morrer (usando defer) */
	defer ServConn.Close()
	for i := 0; i < nServers; i++ {
		defer CliConn[i].Close()
	}

	ch := make(chan string) // Canal que guarda itens lidos do teclado
	go readInput(ch)        // Chamar rotina que "escuta" o teclado (stdin)

	go doServerJob()
	for {
		// Verificar (de forma não bloqueante) se tem algo no stdin (input do terminal)
		select { // Trata o canal de forma não bloqueante. Ou seja, se não tiver nada, não fico bloqueado esperando
		case x, valid := <-ch: // Também tratamos canal fechado (com valid). Mas nesse exemplo, ninguém fecha esse canal.
			if valid {
				fmt.Printf("From keyboard: %s \n", x)
				for j := 0; j < nServers; j++ {
					go doClientJob(j, 100)
				}
			} else {
				fmt.Println("Closed channel!")
			}
		default:
			// Fazer nada!
			// Mas não fica bloqueado esperando o teclado
			time.Sleep(time.Second * 1)
		}

		// Esperar um pouco
		time.Sleep(time.Second * 1)
	}
}
