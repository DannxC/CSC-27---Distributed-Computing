package main

import (
	"fmt"
	"net"
	"os"
)

func CheckError(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(0)
	}
}

func main() {
	Address, err := net.ResolveUDPAddr("udp", ":10001")
	CheckError(err)

	Connection, err := net.ListenUDP("udp", Address)
	CheckError(err)

	defer Connection.Close()
	for {
		// Loop infinito para receber mensagem e escrever todo conteúdo (processo que enviou,
		// relógio recebido e texto) na tela

		// IMPLEMENTAÇÃO
	}
}
