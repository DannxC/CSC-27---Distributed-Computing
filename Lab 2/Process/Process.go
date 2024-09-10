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
	ProcessId int    // Devemos usar letra maiúscula para definir os campos da struct.
	Text      string // Assim eles serão exportados, ou seja, Vistos por outras packages (incluindo o json).
	Clock     int    // Clock da mensagem
}

// Mensagem que será enviada para o recurso compartilhado
type MessageToSharedResource struct {
	ProcessId int    // Processo que enviou a mensagem
	Text      string // Texto da mensagem
	Clock     int    // Clock da mensagem
}

// Variáveis globais interessantes para o procseso

// Deste processo
var processId int    // Id do processo atual
var clock int        // Clock do processo atual (logical scalar clock)
var requestClock int // Clock do request, atualizado ao entrar no WANTED, usado para comparar com requests recebidos
var state int        // Estado do processo: RELEASED, WANTED ou HELD (Ricart-Agrawala)

var replyMap map[int]bool  // Mapa para rastrear quais processos já enviaram reply
var requestQueue []Message // Fila de requests recebidos e não atendidos
var doneReplying chan bool // Canal de sinalização para garantir a execução atômica de releaseCriticalSection

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
		color.Set(color.FgRed)
		fmt.Println("Error: ", err)
		color.Unset()
		os.Exit(0)
	}
}

func PrintError(err error) {
	if err != nil {
		color.Set(color.FgRed)
		fmt.Println("Error: ", err)
		color.Unset()
	}
}

// Função para imprimir o estado atual com cores e formatação
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

	// Formatação do estado com alinhamento central
	color.Set(color.FgHiWhite)
	fmt.Print("\n+------------------+\n")
	fmt.Printf("|     ")
	stateColor.Printf("%-8s", stateStr) // Exibe o estado com cor e alinhado
	color.Set(color.FgHiWhite)
	fmt.Print("     |\n")
	fmt.Print("+------------------+\n")
	color.Unset() // Reseta cor para o padrão
}

func doServerJob() {
	buf := make([]byte, 1024)
	for {
		// Ler (uma vez somente) da conexão UDP a mensagem
		n, _, err := ServConn.ReadFromUDP(buf)
		PrintError(err)

		// Deserializa a mensagem recebida
		var receivedMsg Message
		err = json.Unmarshal(buf[:n], &receivedMsg)
		PrintError(err)

		if receivedMsg.Text == "request" {
			// Tratar a mensagem de request
			handleRequest(receivedMsg)

		} else if receivedMsg.Text == "reply" {
			// Tratar a mensagem de reply
			handleReply(receivedMsg)
		}
	}
}

func handleRequest(receivedMsg Message) {
	// Atualiza o clock lógico com base na mensagem recebida
	clock = max(clock, receivedMsg.Clock) + 1

	// Escrever na tela a msg recebida (indicando o endereço de quem enviou)
	color.Set(color.FgHiWhite)
	fmt.Printf("Received '%s' from %v with clock %d | Clock: %d\n", receivedMsg.Text, receivedMsg.ProcessId, receivedMsg.Clock, clock)
	color.Unset()

	if state == HELD || (state == WANTED && (requestClock < receivedMsg.Clock || (requestClock == receivedMsg.Clock && processId < receivedMsg.ProcessId))) {
		// Enfileirar o request, não responder imediatamente
		color.Set(color.FgHiWhite)
		fmt.Printf("Enfileirando request do processo (id = %d, clock = %d)\n", receivedMsg.ProcessId, receivedMsg.Clock)
		color.Unset()
		requestQueue = append(requestQueue, receivedMsg)
	} else {
		// Responder com um reply imediatamente
		color.Set(color.FgHiWhite)
		fmt.Printf("Enviando reply imediato para o processo %d\n", receivedMsg.ProcessId)
		color.Unset()
		go doClientJob(receivedMsg.ProcessId, "reply", clock) // Sem incrementar o clock
	}

}

func handleReply(receivedMsg Message) {
	if state == WANTED {
		if !replyMap[receivedMsg.ProcessId] {
			replyMap[receivedMsg.ProcessId] = true // Marca que recebeu reply deste processo
			color.Set(color.FgHiWhite)
			fmt.Printf("Reply recebido do processo %d | Total replies: %d\n", receivedMsg.ProcessId, len(replyMap))
			color.Unset()
		} else {
			// Se já recebeu, ignora o reply duplicado
			color.Set(color.FgHiWhite)
			fmt.Printf("Reply duplicado recebido do processo %d\n", receivedMsg.ProcessId)
			color.Unset()
		}
	}
}

func doClientJob(otherProcessId int, text string, clock int) {
	// Cria a mensagem com o texto fornecido (ex: "request" ou "reply")
	msg := Message{ProcessId: processId, Text: text, Clock: clock}

	jsonMsg, err := json.Marshal(msg) // Serializa a mensagem para JSON
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
			color.Set(color.FgHiWhite)
			fmt.Println("x ignorado, processo já está aguardando ou na seção crítica")
			color.Unset()
		} else {
			// Solicitar acesso à seção crítica
			requestCriticalSection()
		}
	} else if id, err := strconv.Atoi(input); err == nil && id == processId {
		// Caso receba o ID do processo, incrementa o clock lógico
		clock++
		color.Set(color.FgHiWhite)
		fmt.Printf("Evento interno! | Clock: %d\n", clock)
		color.Unset()
	} else {
		// Entrada inválida
		color.Set(color.FgRed)
		fmt.Printf("Entrada '%s' inválida. Digite 'x' ou o id do processo (%d).\n", input, processId)
		color.Unset()
	}
}

func requestCriticalSection() {
	// Verifica se ainda está liberando a seção crítica
	if <-doneReplying {
		color.Set(color.FgHiRed)
		fmt.Println("Ainda liberando a seção crítica. Aguardando para entrar no estado WANTED...")
		color.Unset()
		return
	}

	clock++
	requestClock = clock // Armazena o clock atual para os requests
	state = WANTED
	printState() // Imprime o estado após mudança

	color.Set(color.FgHiWhite)
	fmt.Println("Solicitando acesso à seção crítica (enviando requests)... | Clock:", requestClock)
	color.Unset()

	// Enviar requests para todos os outros processos
	for otherProcessId := 1; otherProcessId <= nServers+1; otherProcessId++ {
		if otherProcessId != processId { // Não envia request para si mesmo
			go doClientJob(otherProcessId, "request", requestClock)
		}
	}

	// Esperar até receber N-1 replies
	for len(replyMap) < nServers {
		time.Sleep(500 * time.Millisecond) // Espera um curto intervalo antes de checar novamente
	}

	// Simulação de entrada na seção crítica (recebimento de replies de outros processos)
	enterCriticalSection()
}

func enterCriticalSection() {
	state = HELD
	printState() // Imprime o estado após mudança

	// Obter a porta local da conexão `sharedResourceConn`
	localAddr := sharedResourceConn.LocalAddr().(*net.UDPAddr) // Converte para o tipo *net.UDPAddr
	color.Set(color.FgHiWhite)
	fmt.Printf("Entrando na seção crítica pela porta %d\n", localAddr.Port) // Exibe a porta local usada como cliente
	color.Unset()

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
	// O canal indica que o releaseCriticalSection começou
	doneReplying <- true

	state = RELEASED
	printState() // Imprime o estado após mudança
	color.Set(color.FgHiWhite)
	fmt.Println("Liberando a seção crítica...") // Exibe mensagem de liberação
	color.Unset()

	// Enviar replys para todos os processos na fila (requestQueue)
	for _, request := range requestQueue {
		color.Set(color.FgHiWhite)
		fmt.Printf("Enviando reply para o processo %d\n", request.ProcessId)
		color.Unset()

		// Enviar a mensagem de reply para o processo que estava enfileirado
		go doClientJob(request.ProcessId, "reply", clock)
	}
	requestQueue = []Message{} // Limpar a fila de requests

	replyMap = make(map[int]bool) // Reinicia o mapa de replies

	// Quando terminar, libera o canal
	doneReplying <- false
}

// func printStatePeriodically() {
// 	for {
// 		fmt.Println("Estado atual:", state)
// 		time.Sleep(5 * time.Second) // Pausa por 5 segundos
// 	}
// }

func main() {
	// Parâmetros esperados: id do processo, portas dos processos
	processId, _ = strconv.Atoi(os.Args[1])

	// Verificar se o número de argumentos é suficiente
	if len(os.Args) < 2+processId {
		color.Set(color.FgRed)
		fmt.Println("Erro: Argumentos insuficientes. Verifique o número de processos e portas.")
		color.Unset()
		os.Exit(1)
	}

	// Verificar se nenhuma das portas é a porta do SharedResource (10001)
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == sharedResourcePort {
			color.Set(color.FgRed)
			fmt.Printf("Erro: Porta %s reservada para o SharedResource. Escolha outra porta.\n", sharedResourcePort)
			color.Unset()
			os.Exit(1)
		}
	}

	// Inicializa o clock do processo
	clock = 0
	state = RELEASED // O estado inicial é RELEASED, ou seja, o processo não está na CS nem esperando
	printState()     // Imprime o estado após mudança
	// go printStatePeriodically()

	// Inicializa o map de replies
	replyMap = make(map[int]bool)
	doneReplying = make(chan bool, 1) // Canal para sinalizar a execução atômica de releaseCriticalSection
	doneReplying <- false             // Inicializa o canal com false

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
				// Canal fechado
				color.Set(color.FgRed)
				fmt.Println("Closed Keyboard channel!")
				color.Unset()
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
