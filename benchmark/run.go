package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║           Gomelo Benchmark Suite v1.2.0                      ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println("║ Running performance tests...                                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	benchmarks := []string{
		"MessageEncode",
		"MessageDecode",
		"MessageEncodeDecode",
		"PoolAllocation",
		"PoolNoAlloc",
		"WorkerPoolThroughput",
		"SessionCreation",
		"RouteMatching",
		"JSONSerializeMap",
		"JSONDeserializeMap",
		"BytesBufferWrite",
		"StringConcat",
		"ChannelNonBlocking",
		"HTTPHandler",
	}

	cmd := exec.Command("go", "test", "-bench="+strings.Join(benchmarks, "|"), "-benchtime=1s", "-count=3", "./benchmark/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("Completed in %v\n", elapsed)
	fmt.Println("═══════════════════════════════════════════════════════════════")

	if err != nil {
		fmt.Printf("\nNote: Run from project root with: go test -bench=. ./benchmark/\n")
		fmt.Printf("Some benchmarks may fail if dependencies are missing.\n")
	}
}