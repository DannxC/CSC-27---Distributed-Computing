package mapreduce

import (
	"log"
	"sync"
)

// Schedules map operations on remote workers. This will run until InputFilePathChan
// is closed. If there is no worker available, it'll block.
func (master *Master) schedule(task *Task, proc string, filePathChan chan string) int {
	var (
		wg        sync.WaitGroup
		filePath  string
		worker    *RemoteWorker
		operation *Operation
		counter   int
	)

	log.Printf("Scheduling %v operations\n", proc)

	// Iniciar o goroutine para processar operações falhas antes de despachar as operações iniciais
	go func() {
		for failedOp := range master.failedOperationsChan {
			worker = <-master.idleWorkerChan
			wg.Add(1)
			go master.runOperation(worker, failedOp, &wg)
		}
	}()

	// Despachar as operações iniciais
	counter = 0
	for filePath = range filePathChan {
		operation = &Operation{proc, counter, filePath}
		counter++
		worker = <-master.idleWorkerChan
		wg.Add(1)
		go master.runOperation(worker, operation, &wg)
	}

	// Aguardar todas as operações (iniciais e falhas reprocessadas) serem concluídas
	wg.Wait()

	// Close the failed operations channel after all operations are done
	// close(master.failedOperationsChan)

	log.Printf("%vx %v operations completed\n", counter, proc)
	return counter
}

// runOperation start a single operation on a RemoteWorker and wait for it to return or fail.
func (master *Master) runOperation(remoteWorker *RemoteWorker, operation *Operation, wg *sync.WaitGroup) {
	defer wg.Done() // Assegura que o wg.Done() será chamado ao final da função

	var (
		err  error
		args *RunArgs
	)

	log.Printf("Running %v (ID: '%v' File: '%v' Worker: '%v')\n", operation.proc, operation.id, operation.filePath, remoteWorker.id)

	args = &RunArgs{operation.id, operation.filePath}
	err = remoteWorker.callRemoteWorker(operation.proc, args, new(struct{}))

	if err != nil {
		log.Printf("Operation %v '%v' Failed. Error: %v\n", operation.proc, operation.id, err)
		master.failedWorkerChan <- remoteWorker
		master.failedOperationsChan <- operation // Envia a operação falha para ser reprocessada
	} else {
		master.idleWorkerChan <- remoteWorker
	}
}
