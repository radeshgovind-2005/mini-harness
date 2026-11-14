// Command worker runs a 10-step task whose steps each append one line to a
// side-effect file. Tests start it as a real child process and SIGKILL it.
//
//	worker -db log.db -effects effects.txt [-pause-at N]
//
// With -pause-at N the worker prints "paused N" after step N's effect (before
// its checkpoint) and then blocks, so the parent can kill it at that exact spot.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"mini-harness/eventlog"
	"mini-harness/supervisor"
)

const numSteps = 10

func main() {
	dbPath := flag.String("db", "", "event log sqlite file")
	effects := flag.String("effects", "", "side-effect file")
	pauseAt := flag.Int("pause-at", -1, "block after this step's effect, before its checkpoint")
	flag.Parse()
	if *dbPath == "" || *effects == "" {
		fmt.Fprintln(os.Stderr, "worker: -db and -effects are required")
		os.Exit(2)
	}

	log, err := eventlog.Open(*dbPath, eventlog.SystemClock{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "worker:", err)
		os.Exit(1)
	}

	task := supervisor.Task{ID: "crash-task"}
	for i := 0; i < numSteps; i++ {
		i := i
		task.Steps = append(task.Steps, supervisor.Step{
			Name: fmt.Sprintf("step%d", i),
			Run: func(ctx context.Context, key string) error {
				if err := supervisor.AppendLineOnce(*effects, key, fmt.Sprintf("line %d", i)); err != nil {
					return err
				}
				if i == *pauseAt {
					fmt.Printf("paused %d\n", i)
					time.Sleep(time.Hour) // the parent kills us here
				}
				return nil
			},
		})
	}

	if err := supervisor.NewRunner(log, "worker").Run(context.Background(), task); err != nil {
		fmt.Fprintln(os.Stderr, "worker:", err)
		os.Exit(1)
	}
	fmt.Println("done")
}
