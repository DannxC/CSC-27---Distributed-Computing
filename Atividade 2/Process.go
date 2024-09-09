package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

// Variáveis globais interessantes para o procseso
// var myPort string // porta do meu servidor (ANTIGO)
var myAddress string // endereço do meu servidor (NOVO)

var nServers int           // qtde de outros processos
var CliConn []*net.UDPConn // vetor com conexões para os servidores
// dor outros processos
var ServConn *net.UDPConn // conexão do meu servidor (onde recebo mensagens dos outros processos)

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
	// Enviar uma mensagem (com valor i) para o servidor do processo otherProcess
	msg := strconv.Itoa(i) // Integer to ASCII (string)
	buf := []byte(msg)
	_, err := CliConn[otherProcess].Write(buf)
	PrintError(err)
}

func initConnections() {
	// myPort = os.Args[1] // ANTIGO
	myAddress = os.Args[1] // NOVO

	nServers = len(os.Args) - 2 // Esse 2 tira o nome ("Process") e tira a primeira porta (que é a minha). As demais portas são dos outros processos
	CliConn = make([]*net.UDPConn, nServers)

	/* Outros códigos para deixar ok a conexão do meu servidor (one recebo msgs). O processo já deve ficar habilitado a receber msgs. */

	// ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort) // ANTIGO
	ServerAddr, err := net.ResolveUDPAddr("udp", myAddress) // NOVO
	CheckError(err)
	ServConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)

	/* Outros códigos para deixar ok a minha conexão com cada servidor dos outros processos. Colocar tais conexões no vetor CliConn. */
	for s := 0; s < nServers; s++ {
		// ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+os.Args[2+s]) // ANTIGO
		ServerAddr, err := net.ResolveUDPAddr("udp", os.Args[2+s]) // NOVO
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

	/* Todo Process fará a mesma coisa: ficar ouvindo mensagens e mandar infinitos i's para os outros processos */
	go doServerJob() // Uma goroutine só para o trabalho do server
	i := 0
	for {
		// Temos que ter uma goroutine para cada trabalho do client, tudo rodando de modo concorrente!
		for j := 0; j < nServers; j++ {
			go doClientJob(j, i)
		}
		// Espera um pouco
		time.Sleep(time.Second * 1)
		i++
	}
}
