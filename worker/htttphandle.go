package worker

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"
	"gitlab.ns/lotus-worker/util"
)

func (wl *Worker) RemoteGetSector(w http.ResponseWriter, r *http.Request) {
	log.Infof("SERVE GET %s", r.URL)
	vars := mux.Vars(r)

	pathtype, ok := vars["type"]
	if !ok {
		log.Errorf("url param type no found")
		w.WriteHeader(500)
		return
	}

	actorIDs, ok := vars["actorid"]
	if !ok {
		log.Errorf("url param actorid no found")
		w.WriteHeader(500)
		return
	}
	actorID, err := strconv.ParseInt(actorIDs, 10, 64)
	if err != nil {
		log.Errorf("%+v", err)
		w.WriteHeader(500)
		return
	}

	sectorNums, ok := vars["sectornum"]
	if !ok {
		log.Errorf("url param sectornum no found")
		w.WriteHeader(500)
		return
	}

	sectorNum, err := strconv.ParseInt(sectorNums, 10, 64)
	if err != nil {
		log.Errorf("%+v", err)
		w.WriteHeader(500)
		return
	}

	taskInfo := util.DbTaskInfo{
		ActorID:   &actorID,
		SectorNum: &sectorNum,
	}
	taskInfos, err := wl.MinerApi.QueryOnly(taskInfo)
	if err != nil || len(taskInfos) <= 0 {
		log.Errorf("taskInfos len %d error %v", len(taskInfos), err)
		w.WriteHeader(500)
		return
	}
	taskInfo = taskInfos[0]
	path := ""
	if pathtype == "cache" {
		path = taskInfo.CacheDirPath
	}
	if pathtype == "seal" {
		path = taskInfo.SealedSectorPath
	}
	stat, err := os.Stat(path)
	if err != nil {
		log.Errorf("%+v", err)
		w.WriteHeader(500)
		return
	}

	if stat.IsDir() {
		tw := tar.NewWriter(w)
		defer tw.Close()
		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Errorf("%+v", err)
			w.WriteHeader(500)
		}
		w.Header().Set("Content-Type", "application/x-tar")
		for _, file := range files {
			h, err := tar.FileInfoHeader(file, "")
			if err != nil {
				log.Errorf("%+v", err)
				w.WriteHeader(500)
				return
			}
			h.Name = filepath.Join(fmt.Sprintf("s-t0%d-%d", actorID, sectorNum), file.Name())

			if err := tw.WriteHeader(h); err != nil {
				log.Errorf("%+v", err)
				w.WriteHeader(500)
				return
			}

			f, err := os.OpenFile(filepath.Join(path, file.Name()), os.O_RDONLY, 644) // nolint
			if err != nil {
				log.Errorf("%+v", err)
				w.WriteHeader(500)
				return
			}

			if _, err := io.Copy(tw, f); err != nil {
				log.Errorf("%+v", err)
				w.WriteHeader(500)
				return
			}

			if err := f.Close(); err != nil {
				log.Errorf("%+v", err)
				w.WriteHeader(500)
				return
			}
		}
	} else {
		rd, err := os.OpenFile(path, os.O_RDONLY, 0644) // nolint
		if err != nil {
			log.Errorf("%+v", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")

		w.WriteHeader(200)
		if _, err := io.Copy(w, rd); err != nil { // TODO: default 32k buf may be too small
			log.Errorf("%+v", err)
			return
		}
	}
}

func (wl *Worker) RemoteDelSector(w http.ResponseWriter, r *http.Request) {
	log.Infof("SERVE Delete %s", r.URL)
	vars := mux.Vars(r)

	pathtype, ok := vars["type"]
	if !ok {
		log.Errorf("url param type no found")
		w.WriteHeader(500)
		return
	}

	actorIDs, ok := vars["actorid"]
	if !ok {
		log.Errorf("url param actorid no found")
		w.WriteHeader(500)
		return
	}
	actorID, err := strconv.ParseInt(actorIDs, 10, 64)
	if err != nil {
		log.Errorf("%+v", err)
		w.WriteHeader(500)
		return
	}

	sectorNums, ok := vars["sectornum"]
	if !ok {
		log.Errorf("url param sectornum no found")
		w.WriteHeader(500)
		return
	}

	sectorNum, err := strconv.ParseInt(sectorNums, 10, 64)
	if err != nil {
		log.Errorf("%+v", err)
		w.WriteHeader(500)
		return
	}

	taskInfo := util.DbTaskInfo{
		ActorID:   &actorID,
		SectorNum: &sectorNum,
	}
	taskInfos, err := wl.MinerApi.QueryOnly(taskInfo)
	if err != nil || len(taskInfos) <= 0 {
		log.Errorf("taskInfos len %d error %v", len(taskInfos), err)
		w.WriteHeader(500)
		return
	}
	taskInfo = taskInfos[0]
	if pathtype == "cache" {
		err = delCache(taskInfo.CacheDirPath)
		if err != nil {
			log.Errorf("delete actorID %d sectorNum %d CachePath %s error %v", *taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.CacheDirPath, err)
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("delete actorID %d sectorNum %d Cachefile %s sc-02-data-layer-* sc-02-data-tree-[cd]*", *taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.CacheDirPath)))
	} else if pathtype == "seal" {
		err = os.Remove(taskInfo.SealedSectorPath)
		if err != nil {
			log.Warnf("actorID %d sectorNum %d sealedPath %s remove error %v", *taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.SealedSectorPath, err)
			w.WriteHeader(500)
			return
		}
		err = os.RemoveAll(taskInfo.CacheDirPath)
		if err != nil {
			log.Warnf("actorID %d sectorNum %d CachePath %s remove error %v", *taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.CacheDirPath, err)
			w.WriteHeader(500)
			return
		}

		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("actorID %d sectorNum %d sealedPath %s CachePath %s removed", *taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.SealedSectorPath, taskInfo.CacheDirPath)))
	} else {
		w.WriteHeader(200)
		w.Write([]byte("undefine option type, 'seal' or 'cache'"))
	}
}

func delCache(cachepath string) error {
	cmd := exec.Command("/bin/bash", "-c", "rm -f "+filepath.Join(cachepath, `sc-02-data-tree-[cd]*`))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		return err
	}

	cmd = exec.Command("/bin/bash", "-c", "rm -f "+filepath.Join(cachepath, `sc-02-data-layer-*`))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return err
	}
	return err
}
