package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
)

type TailscaleStatus struct {
	Peer map[string]struct {
		TailscaleIPs []string `json:"TailscaleIPs"`
		HostName     string   `json:"HostName"`
	} `json:"Peer"`
	Self struct {
		TailscaleIPs []string `json:"TailscaleIPs"`
	} `json:"Self"`
}

// Fetch Tailscale IPs (peers + self)
func GetTailscaleIPs() ([]string, error) {
	cmd := exec.Command("tailscale", "status", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error executing tailscale command: %w", err)
	}

	var status TailscaleStatus
	err = json.Unmarshal(output, &status)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	var ips []string
	for _, peer := range status.Peer {
		if len(peer.TailscaleIPs) > 0 {
			ips = append(ips, peer.TailscaleIPs[0])
		}
	}

	if len(status.Self.TailscaleIPs) > 0 {
		ips = append(ips, status.Self.TailscaleIPs[0])
	}

	return ips, nil
}

// Upload a file to IPFS
func uploadFileToIPFS(sh *shell.Shell, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	cid, err := sh.Add(file)
	if err != nil {
		return "", fmt.Errorf("failed to upload file to IPFS: %v", err)
	}

	return cid, nil
}

// Send CIDs to the server (no wait for response)
func sendData(ip, hash1, hash2 string) error {
	conn, err := net.Dial("tcp", ip+":8080")
	if err != nil {
		return fmt.Errorf("server %s is not reachable: %w", ip, err)
	}
	defer conn.Close()

	_, err = fmt.Fprintf(conn, hash1+" "+hash2+"\n")
	if err != nil {
		return fmt.Errorf("error sending hash: %w", err)
	}

	// No need to wait for the response anymore
	// Simply closing the connection after sending data
	return nil
}

// Main logic for periodically sending random files (1 algorithm and 1 data file)
func periodicFileBroadcast(sh *shell.Shell, algoDir, dataDir string, interval time.Duration, numFiles int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		// Get the list of Tailscale IPs (peers)
		ips, err := GetTailscaleIPs()
		if err != nil {
			fmt.Println("Error getting Tailscale IPs:", err)
			continue
		}

		// Remove duplicate IPs
		uniqueIPs := make(map[string]bool)
		for _, ip := range ips {
			uniqueIPs[ip] = true
		}

		var allIPs []string
		for ip := range uniqueIPs {
			allIPs = append(allIPs, ip)
		}

		// Select a random algorithm file (numbered 1 to numFiles)
		num1 := rand.Intn(numFiles) + 1
		algoFile := fmt.Sprintf("a_star_%d.py", num1)

		// Select a random data file (numbered 1 to numFiles)
		num2 := rand.Intn(numFiles) + 1
		dataFile := fmt.Sprintf("graph_%d.txt", num2)

		// Open the selected algorithm file
		afile, err := os.Open(filepath.Join(algoDir, algoFile))
		if err != nil {
			fmt.Printf("Error opening algorithm file %s: %v\n", algoFile, err)
			continue
		}
		defer afile.Close()

		// Upload the algorithm file to IPFS
		cidalgo, err := sh.Add(afile)
		if err != nil {
			fmt.Printf("Error uploading algorithm file %s to IPFS: %v\n", algoFile, err)
			continue
		}
		fmt.Printf("Uploaded algorithm file %s to IPFS. CID: %s\n", algoFile, cidalgo)

		// Open the selected data file
		dfile, err := os.Open(filepath.Join(dataDir, dataFile))
		if err != nil {
			fmt.Printf("Error opening data file %s: %v\n", dataFile, err)
			continue
		}
		defer dfile.Close()

		// Upload the data file to IPFS
		ciddata, err := sh.Add(dfile)
		if err != nil {
			fmt.Printf("Error uploading data file %s to IPFS: %v\n", dataFile, err)
			continue
		}
		fmt.Printf("Uploaded data file %s to IPFS. CID: %s\n", dataFile, ciddata)

		// Use a WaitGroup to handle concurrent sending of CIDs to all servers
		var wg sync.WaitGroup

		for _, ip := range allIPs {
			wg.Add(1) // Increment the WaitGroup counter
			go func(ip, algoCID, dataCID string) {
				defer wg.Done() // Decrement the counter when the goroutine completes
				fmt.Printf("Sending CIDs to server at %s:8080\n", ip)
				err := sendData(ip, algoCID, dataCID)
				if err != nil {
					fmt.Printf("Error sending data to %s: %v\n", ip, err)
				} else {
					fmt.Printf("Successfully sent data to %s\n", ip)
				}
			}(ip, cidalgo, ciddata) // Pass variables as arguments to avoid closure issues
		}

		// Wait for all goroutines to finish
		wg.Wait()
		fmt.Println("All data sent for this interval.")
	}
}

func main() {
	sh := shell.NewShell("localhost:5001")

	algoDir := "data/algorithms"
	dataDir := "data/datasets"

	num_files := 1               // Total number of algorithm/data files available
	interval := 30 * time.Second // Interval between file broadcasts

	// Start periodic file broadcast
	periodicFileBroadcast(sh, algoDir, dataDir, interval, num_files)
}
