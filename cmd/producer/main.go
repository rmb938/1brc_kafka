package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kslog"
	"golang.org/x/sys/unix"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{ /*Level: slog.LevelDebug*/ }))

	slog.SetDefault(logger)

	kafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers("localhost:16500"),

		// TODO: we probably need to continue to tweak these to be optimal
		//	producer still spends a lot of time waiting to produce
		kgo.MaxBufferedRecords(1_000_000),
		kgo.ProducerBatchMaxBytes(16_000_000),

		// disable compression so all kafka implementations are equal in compression
		kgo.ProducerBatchCompression(kgo.NoCompression()),

		kgo.WithLogger(kslog.New(slog.Default())),
	)
	if err != nil {
		slog.Error("error creating kafka client", "error", err)
		os.Exit(1)
	}

	err = kafkaClient.Ping(context.Background())
	if err != nil {
		slog.Error("error pinging kafka", "error", err)
		os.Exit(1)
	}

	slog.Info("Loading File")

	measurementsFile, err := os.Open("1brc/measurements.txt")
	if err != nil {
		slog.Error("error opening file", "error", err)
		os.Exit(1)
	}

	measurementsFileFI, err := measurementsFile.Stat()
	if err != nil {
		slog.Error("error statting file", "error", err)
		os.Exit(1)
	}

	measurementsMapped, err := unix.Mmap(int(measurementsFile.Fd()), 0, int(measurementsFileFI.Size()), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		slog.Error("error mapping file", "error", err)
		os.Exit(1)
	}

	// always defer unmap so we free up memory
	defer func() {
		unix.Munmap(measurementsMapped)
		if err != nil {
			slog.Error("error unmapping file", "error", err)
			os.Exit(1)
		}
	}()

	chunkCount := runtime.NumCPU()
	chunkSize := len(measurementsMapped) / chunkCount

	chunks := make([]int, 0, chunkCount)
	offset := 0
	for offset < len(measurementsMapped) {
		offset += chunkSize
		if offset >= len(measurementsMapped) {
			chunks = append(chunks, len(measurementsMapped))
			break
		}

		nlOffset := bytes.IndexByte(measurementsMapped[offset:], '\n')
		if nlOffset == -1 {
			chunks = append(chunks, len(measurementsMapped))
			break
		} else {
			offset += nlOffset + 1
			chunks = append(chunks, offset)
		}
	}

	var wg sync.WaitGroup
	wg.Add(chunkCount)

	start := 0
	for i, chunk := range chunks {
		go func(data []byte, i int) {

			offset := 0
			for {
				nlOffset := bytes.IndexByte(data[offset:], '\n')
				var line []byte
				if nlOffset == -1 {
					line = data[offset:]
				} else {
					line = data[offset : offset+nlOffset]
				}

				processLine(line, kafkaClient)

				if nlOffset == -1 {
					break
				}
				offset += nlOffset + 1
			}

			wg.Done()
		}(measurementsMapped[start:chunk], i)
		start = chunk
	}
	wg.Wait()

}

func processLine(line []byte, kafkaClient *kgo.Client) {
	colonOffset := bytes.IndexByte(line, ';')
	record := kgo.KeySliceRecord(line[:colonOffset], line[1+colonOffset:])
	record.Topic = "1brc"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	kafkaClient.Produce(ctx, record, func(r *kgo.Record, err error) {
		defer cancel()
		if err != nil {
			slog.Info("error producing record", "error", err)
			os.Exit(1)
		}
	})
}
