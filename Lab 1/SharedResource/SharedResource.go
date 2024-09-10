package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

type MessageToSharedResource struct {
	ProcessId int    // Processo que enviou a mensagem
	Text      string // Texto da mensagem
	Clock     int    // Clock da mensagem
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

func main() {
	Address, err := net.ResolveUDPAddr("udp", ":10001")
	CheckError(err)

	Connection, err := net.ListenUDP("udp", Address)
	CheckError(err)

	defer Connection.Close()
	buf := make([]byte, 1024)
	for {
		// Loop infinito para receber mensagem e escrever todo conteúdo (processo que enviou,
		// relógio recebido e texto) na tela

		// Ler (uma vez somente) da conexão UDP a mensagem
		n, addr, err := Connection.ReadFromUDP(buf)
		if err != nil {
			PrintError(err)
			continue // Continua a escutar novas mensagens, ignorando o erro
		}

		// Deserializa a mensagem recebida
		var receivedMsg MessageToSharedResource
		err = json.Unmarshal(buf[:n], &receivedMsg)
		if err != nil {
			PrintError(err)
			continue // Continua a escutar novas mensagens, ignorando o erro
		}

		// Extrai informações da mensagem recebida
		processId := receivedMsg.ProcessId
		text := receivedMsg.Text
		clock := receivedMsg.Clock

		// Escreve na tela as informações da mensagem recebida
		fmt.Printf("Processo %d (%s) enviou a mensagem \"%s\" com clock %d\n", processId, addr, text, clock)

		// Espera 1 segundo para retornar a mensagem recebida
		time.Sleep(1 * time.Second)
	}
}
