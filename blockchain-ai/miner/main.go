package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	shell "github.com/ipfs/go-ipfs-api"
)

type Transaction struct {
	ID   string
	Data string
}

type Block struct {
	PrevHash     string
	Transactions []Transaction
	Nonce        int
	Hash         string
	num_blocks   int
	t_cid        string // IPFS CID of the transaction
}

var (
	transactionBuffer = make(chan Transaction, 100)           // Buffer for dynamically created transactions
	newBlock          = make(chan Block)                      // Channel to broadcast new blocks
	stopMining        = make(chan struct{})                   // Channel to stop the mining process
	target            = big.NewInt(1).Lsh(big.NewInt(1), 245) // Approximate target for ~30 seconds
	ipfsShell         = shell.NewShell("localhost:5001")      // IPFS shell instance
)

// Download file from IPFS.
func downloadFromIPFS(cid, outputPath string) error {
	reader, err := ipfsShell.Cat(cid)
	if err != nil {
		return fmt.Errorf("failed to fetch file from IPFS: %v", err)
	}
	defer reader.Close()

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to write to output file: %v", err)
	}
	return nil
}

// Execute Python script with input data.
func executeScript(scriptPath, dataPath string) (string, error) {
	cmd := exec.Command("python", scriptPath, dataPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("script execution failed: %v, output: %s", err, string(output))
	}
	return string(output), nil
}

// Transaction Processing Thread
func processTransactions(wg *sync.WaitGroup) {
	defer wg.Done()

	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Error starting transaction listener:", err)
		return
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			scanner := bufio.NewScanner(conn)
			for scanner.Scan() {
				message := scanner.Text()
				fmt.Println("Received hashes:", message)

				parts := strings.Split(message, " ")
				if len(parts) != 2 {
					fmt.Println("Invalid message format. Expected '<data_hash> <script_hash>'")
					continue
				}
				scriptHash, dataHash := parts[0], parts[1]

				// Download data and script from IPFS
				dataPath := "data.txt"
				scriptPath := "script.py"

				if err := downloadFromIPFS(dataHash, dataPath); err != nil {
					fmt.Println("Failed to download data:", err)
					continue
				}

				if err := downloadFromIPFS(scriptHash, scriptPath); err != nil {
					fmt.Println("Failed to download script:", err)
					continue
				}

				// Execute the script to produce the transaction
				result, err := executeScript(scriptPath, dataPath)
				if err != nil {
					fmt.Println("Error executing script:", err)
					continue
				}

				// Create a transaction from the result
				transaction := Transaction{
					ID:   generateTransactionID(result),
					Data: result,
				}

				// Add the transaction to the buffer
				transactionBuffer <- transaction
				fmt.Println("Transaction created and added to buffer:", transaction)
			}
		}(conn)
	}
}

// Mining Thread
func startMining(prevHash string, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-stopMining:
			fmt.Println("Stopping mining thread...")
			return
		default:
			// Wait for exactly 3 transactions
			transactions := make([]Transaction, 0, 3)
			for len(transactions) < 3 {
				tx := <-transactionBuffer // This blocks until a transaction is available
				transactions = append(transactions, tx)
				fmt.Println("Added transaction to block:", tx)
			}

			// Perform proof of work
			nonce := 0
			for {
				select {
				case <-stopMining:
					return
				default:
					blockData := fmt.Sprintf("%s:%v:%d", prevHash, transactions, nonce)
					hash := sha256.Sum256([]byte(blockData))
					hashInt := new(big.Int).SetBytes(hash[:])
					if hashInt.Cmp(target) == -1 {
						block := Block{
							PrevHash:     prevHash,
							Transactions: transactions,
							Nonce:        nonce,
							Hash:         hex.EncodeToString(hash[:]),
						}
						fmt.Println("Mined a new block:", block.Hash)
						newBlock <- block
						return
					}
					nonce++
				}
			}
		}
	}
}

// Block Reception and Validation Thread
func receiveAndValidateBlocks(wg *sync.WaitGroup) {
	defer wg.Done()

	ln, err := net.Listen("tcp", ":8081")
	if err != nil {
		fmt.Println("Error starting block listener:", err)
		return
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			scanner := bufio.NewScanner(conn)
			for scanner.Scan() {
				blockData := scanner.Text()
				fmt.Println("Received block:", blockData)
				close(stopMining) // Stop mining on receiving a block
			}
		}(conn)
	}
}

// Helper Functions
func generateTransactionID(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Main Function
func main() {
	var wg sync.WaitGroup

	prevHash := "genesis" // Replace with the actual previous hash or genesis block hash

	wg.Add(1)
	go startMining(prevHash, &wg)

	wg.Add(1)
	go receiveAndValidateBlocks(&wg)

	wg.Add(1)
	go processTransactions(&wg)

	wg.Wait()
}
