package worker

import (
	"cube/task"
	"errors"
	"fmt"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"log"
	"time"
)

type Worker struct {
	Name      string
	Queue     queue.Queue
	Db        map[uuid.UUID]*task.Task
	TaskCount int
	Stats     *Stats
}

func (w *Worker) CollectStats() {
	for {
		log.Println("collecting stats")
		w.Stats = GetStats()
		w.Stats.TaskCount = w.TaskCount
		time.Sleep(15 * time.Second)
	}
}

func (w *Worker) RunTask() task.DockerResult {
	t := w.Queue.Dequeue()
	if t == nil {
		log.Println("No tasks in the queue")
		return task.DockerResult{Error: nil}
	}

	taskQueued := t.(task.Task)

	taskPersisted := w.Db[taskQueued.ID]
	if taskPersisted == nil {
		taskPersisted = &taskQueued
		w.Db[taskQueued.ID] = &taskQueued
	}

	var result task.DockerResult
	if task.ValidStateTransition(taskPersisted.State, taskQueued.State) {
		switch taskQueued.State {
		case task.Scheduled:
			result = w.StartTask(taskQueued)
		case task.Completed:
			result = w.StopTask(taskQueued)
		default:
			result.Error = errors.New("We should not get here")
		}
	} else {
		err := fmt.Errorf("Invalid transition from %v to %v", taskPersisted.State, taskQueued.State)
		result.Error = err
	}
	return result
}

func (w *Worker) AddTask(t task.Task) {
	w.Queue.Enqueue(t)
}

func (w *Worker) StartTask(t task.Task) task.DockerResult {
	t.StartTime = time.Now().UTC()
	config := task.NewConfig(&t)
	d := task.NewDocker(config)
	result := d.Run()
	if result.Error != nil {
		log.Printf("Error running task %v: %v\n", t.ID, result.Error)
		t.State = task.Failed
		w.Db[t.ID] = &t
		return result
	}
	t.ContainerID = result.ContainerId
	t.State = task.Running
	w.Db[t.ID] = &t

	return result
}

func (w *Worker) StopTask(t task.Task) task.DockerResult {
	config := task.NewConfig(&t)
	d := task.NewDocker(config)

	result := d.Stop(t.ContainerID)
	if result.Error != nil {
		log.Printf("Error stopping container %v: %v\n", t, result.Error)
	}
	t.FinishTime = time.Now().UTC()
	t.State = task.Completed
	w.Db[t.ID] = &t
	log.Printf("Stopped and removed container %v for task %v\n", t.ContainerID, t.ID)

	return result
}

func (w *Worker) GetTasks() []*task.Task {
	tasks := []*task.Task{}
	for _, t := range w.Db {
		tasks = append(tasks, t)
	}
	return tasks
}

func (w *Worker) runTask() task.DockerResult {
	t := w.Queue.Dequeue()
	if t == nil {
		log.Println("No tasks in the queue")
		return task.DockerResult{Error: nil}
	}

	taskQueued := t.(task.Task)
	fmt.Printf("Found task in queue: %v:\n", taskQueued)

	taskPersisted := w.Db[taskQueued.ID]
	if taskPersisted == nil {
		taskPersisted = &taskQueued
		w.Db[taskQueued.ID] = &taskQueued
	}

	var result task.DockerResult
	if task.ValidStateTransition(taskPersisted.State, taskQueued.State) {
		switch taskQueued.State {
		case task.Scheduled:
			result = w.StartTask(taskQueued)
		case task.Completed:
			result = w.StopTask(taskQueued)
		default:
			fmt.Printf("This is a mistake. taskPersisted: %v, taskQueued: %v\n", taskPersisted, taskQueued)
			result.Error = errors.New("We should not get here")
		}
	} else {
		err := fmt.Errorf("Invalid transition from %v to %v", taskPersisted.State, taskQueued.State)
		result.Error = err
		return result
	}
	return result
}

func (w *Worker) RunTasks() {
	for {
		if w.Queue.Len() != 0 {
			result := w.runTask()
			if result.Error != nil {
				log.Printf("Error running task: %v\n", result.Error)
			}
		} else {
			log.Printf("No tasks to process currently.\n")
		}
		log.Println("Sleeping for 10 seconds.")
		time.Sleep(10 * time.Second)
	}

}

func (w *Worker) InspectTask(t task.Task) task.DockerInspectResponse {
	config := task.NewConfig(&t)
	d := task.NewDocker(config)
	return d.Inspect(t.ContainerID)
}

func (w *Worker) UpdateTasks() {
	for {
		log.Println("Checking status of tasks")
		w.updateTasks()
		log.Println("Task updates completed")
		log.Println("Sleeping for 15 seconds")
		time.Sleep(15 * time.Second)
	}
}

func (w *Worker) updateTasks() {
	for id, t := range w.Db {
		if t.State == task.Running {
			resp := w.InspectTask(*t)
			if resp.Error != nil {
				fmt.Printf("ERROR: %v\n", resp.Error)
			}

			if resp.Container == nil {
				log.Printf("No container for running task %s\n")
				w.Db[id].State = task.Failed
			}

			if resp.Container.State.Status == "exited" {
				log.Printf("Container for task %s in non-running state %s", id, resp.Container.State.Status)
				w.Db[id].State = task.Failed
			}

			w.Db[id].HostPorts = resp.Container.NetworkSettings.Ports
		}
	}
}
