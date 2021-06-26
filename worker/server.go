package worker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"gitlab.ns/lotus-worker/util"
)

var logserver = logging.Logger("server")
var bin = ""

func init() {
	var err error
	bin, err = getCurrentDirectory()
	if err != nil {
		panic(err)
	}
}

func (w *Worker) AddToServer(actorID int64, tasks []string) error {
	w.ActorTask[actorID] = tasks
	return nil
}

func (w *Worker) DeleteFromServer(actorID int64) error {
	delete(w.ActorTask, actorID)
	return nil
}

func (w *Worker) DisplayServer(actorID int64) (interface{}, error) {
	if actorID != -1 {
		return w.ActorTask[actorID], nil
	}
	return w.ActorTask, nil
}

func (w *Worker) Server() {
	for {
		for actorID, tasks := range w.ActorTask {
			tactorID := actorID
			ttasks := tasks
			go func() {
				for _, task := range ttasks {
					ttask := task
					switch ttask {
					case util.PC1:
						ci, err := w.ProcessPrePhase1(tactorID, -1, bin)
						if err != nil {
							logserver.Errorf("process actorid %d task type %s error %v", tactorID, ttask, err)
							continue
						}
						logserver.Infof("process actorid %d task type %s pid info %v", tactorID, ttask, ci)
					case util.PC2:
						ci, err := w.ProcessPrePhase2(tactorID, -1, bin)
						if err != nil {
							logserver.Errorf("process actorid %d task type %s error %v", tactorID, ttask, err)
							continue
						}
						logserver.Infof("process actorid %d task type %s pid info %v", tactorID, ttask, ci)
					case util.C1:
						ci, err := w.ProcessCommitPhase1(tactorID, -1, bin)
						if err != nil {
							logserver.Errorf("process actorid %d task type %s error %v", tactorID, ttask, err)
							continue
						}
						logserver.Infof("process actorid %d task type %s pid info %v", tactorID, ttask, ci)
					case util.C2:
						ci, err := w.ProcessCommitPhase2(tactorID, -1, bin)
						if err != nil {
							logserver.Errorf("process actorid %d task type %s error %v", tactorID, ttask, err)
							continue
						}
						logserver.Infof("process actorid %d task type %s pid info %v", tactorID, ttask, ci)
					}
				}
			}()
		}
		time.Sleep(30 * time.Second)
	}
}

func getCurrentDirectory() (string, error) {
	file, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	path, err := filepath.Abs(file)
	if err != nil {
		return "", err
	}
	return strings.Replace(path, "\\", "/", -1), nil
}
