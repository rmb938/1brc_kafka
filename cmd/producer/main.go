package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
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

	offset := 0
	for {
		nlPos := bytes.IndexByte(measurementsMapped[offset:], '\n')
		var line []byte
		if nlPos == -1 {
			line = measurementsMapped[offset:]
		} else {
			line = measurementsMapped[offset : offset+nlPos]
		}

		colonPos := bytes.IndexByte(line, ';')
		record := kgo.KeySliceRecord(line[:colonPos], line[1+colonPos:])
		record.Topic = "1brc"

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rs := kafkaClient.ProduceSync(ctx, record)
		err = rs.FirstErr()
		if err != nil {
			slog.Info("error producing record", "error", err)
			os.Exit(1)
		}

		if nlPos == -1 {
			break
		}

		offset += nlPos + 1
	}

}
