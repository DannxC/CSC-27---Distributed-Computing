package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

type Message struct {
	Code int    // Devemos usar letra maiúscula para definir os campos da struct.
	Text string // Assim eles serão exportados, ou seja, Vistos por outras packages (incluindo o json).

	MsgClock int // Clock da mensagem
}

// Variáveis globais interessantes para o procseso

// Deste processo
var processId int // Id do processo atual
var clock int     // Clock do processo atual (logical scalar clock)

var myPort string          // porta do meu servidor
var CliConn []*net.UDPConn // vetor com conexões para os servidores

// Dos outros processos
var nServers int          // qtde de outros processos
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

		// Deserializa a mensagem recebida
		var receivedMsg Message
		err = json.Unmarshal(buf[:n], &receivedMsg)
		PrintError(err)

		// Atualiza o clock lógico com base na mensagem recebida
		clock = max(clock, receivedMsg.MsgClock) + 1

		// Escrever na tela a msg recebida (indicando o endereço de quem enviou)
		fmt.Printf("Received %s from %v | Clock: %d\n", receivedMsg.Text, addr, clock)
	}
}

func doClientJob(otherProcess int, i int) {
	// Incrementa o clock ao enviar a mensagem
	clock++

	msg := Message{i, "Teste", clock} // A mensagem será fixa. Sempre com Code "i" e Text "Teste", além do clock atual
	jsonMsg, err := json.Marshal(msg) // Agora json fará parte dos imports
	PrintError(err)

	_, err = CliConn[otherProcess].Write(jsonMsg)
	PrintError(err)
}

func initConnections() {
	myPort = os.Args[1+processId] // Porta do meu servidor
	nServers = len(os.Args) - 3   // Calcula o número de servidores (processos restantes)
	CliConn = make([]*net.UDPConn, nServers)

	/* Outros códigos para deixar ok a conexão do meu servidor (onde recebo msgs). O processo já deve ficar habilitado a receber msgs. */

	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort)
	CheckError(err)
	ServConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)

	/* Outros códigos para deixar ok a minha conexão com cada servidor dos outros processos. Colocar tais conexões no vetor CliConn. */
	connIndex := 0
	for s := 0; s < nServers+1; s++ {
		if s == processId-1 { // Pula a própria porta
			continue
		}

		ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+os.Args[2+s])
		CheckError(err)

		/* Aqui não foi definido o endereço do cliente. Usando nil, o próprio sistema escolhe. */
		Conn, err := net.DialUDP("udp", nil, ServerAddr)
		CliConn[connIndex] = Conn // Armazena no vetor CliConn na posição correta
		connIndex++               // Incrementa o índice de CliConn
		CheckError(err)
	}
}

func main() {
	// Parâmetros esperados: id do processo, portas dos processos
	processId, _ = strconv.Atoi(os.Args[1])

	// Verificar se o número de argumentos é suficiente
	if len(os.Args) < 2+processId {
		fmt.Println("Erro: Argumentos insuficientes. Verifique o número de processos e portas.")
		os.Exit(1)
	}

	// Inicializa o clock do processo
	clock = 0

	// Inicializa as conexões
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
