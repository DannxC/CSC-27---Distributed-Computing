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

// Definição de estados usando iota para simular enum
const (
	RELEASED = iota // 0: O processo não está tentando acessar a CS
	WANTED          // 1: O processo quer acessar a CS
	HELD            // 2: O processo está na CS
)

// Constante para a porta do SharedResource
const sharedResourcePort = "10001"

// Definição de Structs

// Mensagem que será enviada entre os processos
type Message struct {
	Code     int    // Devemos usar letra maiúscula para definir os campos da struct.
	Text     string // Assim eles serão exportados, ou seja, Vistos por outras packages (incluindo o json).
	MsgClock int    // Clock da mensagem
}

// Mensagem que será enviada para o recurso compartilhado
type MessageToSharedResource struct {
	ProcessId int    // Processo que enviou a mensagem
	Text      string // Texto da mensagem
	Clock     int    // Clock da mensagem
}

// Variáveis globais interessantes para o procseso

// Deste processo
var processId int // Id do processo atual
var clock int     // Clock do processo atual (logical scalar clock)
var state int     // Estado do processo: RELEASED, WANTED ou HELD (Ricart-Agrawala)

var myPort string // porta do meu servidor
// var CliConn []*net.UDPConn // vetor com conexões para os servidores // ANTIGO
var CliConn map[int]*net.UDPConn    // Map de conexões com outros processos (mapeia ID do processo para a conexão UDP) // NOVO
var sharedResourceConn *net.UDPConn // Conexão com o SharedResource (seção crítica)

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

// Função para imprimir o estado atual
func printState() {
	var stateStr string
	switch state {
	case RELEASED:
		stateStr = "RELEASED"
	case WANTED:
		stateStr = "WANTED"
	case HELD:
		stateStr = "HELD"
	}
	fmt.Printf("| state = %s\n", stateStr)
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
		fmt.Printf("Received '%s' from %v | Clock: %d\n", receivedMsg.Text, addr, clock)
	}
}

func doClientJob(otherProcessId int, i int, clock int) {
	msg := Message{i, "Teste", clock} // A mensagem será fixa. Sempre com Code "i" e Text "Teste", além do clock atual
	jsonMsg, err := json.Marshal(msg) // Agora json fará parte dos imports
	PrintError(err)

	// Envia a mensagem para o processo correspondente no map
	conn := CliConn[otherProcessId]
	_, err = conn.Write(jsonMsg)
	PrintError(err)
}

func initConnections() {
	myPort = os.Args[1+processId]        // Porta do meu servidor
	nServers = len(os.Args) - 3          // Calcula o número de servidores (processos restantes)
	CliConn = make(map[int]*net.UDPConn) // Inicializa o map de conexões

	/* Outros códigos para deixar ok a conexão do meu servidor (onde recebo msgs). O processo já deve ficar habilitado a receber msgs. */

	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort)
	CheckError(err)
	ServConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)

	/* Inicializa a conexão com o SharedResource (seção crítica) */
	sharedResourceAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+sharedResourcePort)
	CheckError(err)
	sharedResourceConn, err = net.DialUDP("udp", nil, sharedResourceAddr) // Criar conexão global com o SharedResource
	CheckError(err)

	/* Outros códigos para deixar ok a minha conexão com cada servidor dos outros processos. Colocar tais conexões no vetor CliConn. */
	for s := 0; s < nServers+1; s++ {
		if s == processId-1 { // Pula a própria porta
			continue
		}

		// ID do processo para o qual estamos conectando
		otherProcessId := s + 1

		// Resolve o endereço do servidor
		ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+os.Args[2+s])
		CheckError(err)

		// Estabelece a conexão UDP e armazena no map usando o ID do processo
		Conn, err := net.DialUDP("udp", nil, ServerAddr)
		CliConn[otherProcessId] = Conn
		CheckError(err)
	}
}

func handleKeyboardInput(input string) {
	if input == "x" {
		// Se o processo já está na CS ou aguardando, ignorar "x"
		if state == WANTED || state == HELD {
			fmt.Println("x ignorado, processo já está aguardando ou na seção crítica")
		} else {
			// Solicitar acesso à seção crítica
			fmt.Println("Solicitando acesso à seção crítica")
			requestCriticalSection()
		}
	} else if id, err := strconv.Atoi(input); err == nil && id == processId {
		// Caso receba o ID do processo, incrementa o clock lógico
		clock++
		fmt.Printf("Relógio incrementado para %d\n", clock)
	} else {
		// Entrada inválida
		fmt.Printf("Entrada '%s' inválida. Digite 'x' ou o id do processo (%d).\n", input, processId)
	}
}

func requestCriticalSection() {
	state = WANTED

	// Simular o envio de mensagens de "request" para outros processos
	fmt.Println("Enviando pedidos de seção crítica para outros processos...")
	// Aqui você poderia implementar o envio de requests via UDP para outros processos
	// usando a função doClientJob()

	// Simulação de entrada na seção crítica (recebimento de replies de outros processos)
	enterCriticalSection()
}

func enterCriticalSection() {
	state = HELD

	// Obter a porta local da conexão `sharedResourceConn`
	localAddr := sharedResourceConn.LocalAddr().(*net.UDPAddr)              // Converte para o tipo *net.UDPAddr
	fmt.Printf("Entrando na seção crítica pela porta %d\n", localAddr.Port) // Exibe a porta local usada como cliente

	// Criar a mensagem para enviar ao SharedResource
	msg := MessageToSharedResource{
		ProcessId: processId,
		Text:      "Hello, CS!",
		Clock:     clock,
	}

	// Serializar a mensagem para JSON
	jsonMsg, err := json.Marshal(msg)
	CheckError(err)

	// Enviar a mensagem ao SharedResource usando a conexão global `sharedResourceConn`
	_, err = sharedResourceConn.Write(jsonMsg)
	CheckError(err)

	// Simular o uso da CS (dormir por 2 segundos)
	time.Sleep(2 * time.Second)

	// Após usar a CS, liberar
	releaseCriticalSection()
}

func releaseCriticalSection() {
	state = RELEASED
	fmt.Println("Liberando a seção crítica")
	// Simular o envio de mensagens de liberação (RELEASE) para outros processos
	// Você pode chamar doClientJob() aqui para simular a liberação da CS para outros processos
}

func main() {
	// Parâmetros esperados: id do processo, portas dos processos
	processId, _ = strconv.Atoi(os.Args[1])

	// Verificar se o número de argumentos é suficiente
	if len(os.Args) < 2+processId {
		fmt.Println("Erro: Argumentos insuficientes. Verifique o número de processos e portas.")
		os.Exit(1)
	}

	// Verificar se nenhuma das portas é a porta do SharedResource (10001)
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == sharedResourcePort {
			fmt.Printf("Erro: Porta %s reservada para o SharedResource. Escolha outra porta.\n", sharedResourcePort)
			os.Exit(1)
		}
	}

	// Inicializa o clock do processo
	clock = 0
	state = RELEASED // O estado inicial é RELEASED, ou seja, o processo não está na CS nem esperando

	// Inicializa as conexões
	initConnections()

	/* O fechamento de conexões deve ficar aqui, assim só fecha conexão quando a main morrer (usando defer) */
	defer ServConn.Close()
	defer sharedResourceConn.Close()
	for _, conn := range CliConn {
		defer conn.Close() // Fecha cada conexão armazenada no map
	}

	ch := make(chan string) // Canal que guarda itens lidos do teclado
	go readInput(ch)        // Chamar rotina que "escuta" o teclado (stdin)

	go doServerJob()
	for {
		// Verificar (de forma não bloqueante) se tem algo no stdin (input do terminal)
		select { // Trata o canal de forma não bloqueante. Ou seja, se não tiver nada, não fico bloqueado esperando
		case input, valid := <-ch: // Também tratamos canal fechado (com valid). Mas nesse exemplo, ninguém fecha esse canal.
			if valid {
				handleKeyboardInput(input) // Função para tratar entrada do teclado
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
