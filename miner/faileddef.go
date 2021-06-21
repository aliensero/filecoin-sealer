package miner

import (
	"github.com/jinzhu/gorm"
	"gitlab.ns/lotus-worker/util"
)

func InitDbTaskFailed(db *gorm.DB) error {

	taskFaileds := []util.DbTaskFailed{
		{
			TaskType:    util.PC1,
			ErrOutline:  "timeout",
			RetryCnt:    5,
			ExecType:    util.EXECRETRY,
			UpdateState: util.RETRY,
		},

		{
			TaskType:    util.PC2,
			ErrOutline:  "timeout",
			RetryCnt:    5,
			ExecType:    util.EXECRETRY,
			UpdateState: util.RETRY,
		},

		{
			TaskType:    util.C1,
			ErrOutline:  "timeout",
			RetryCnt:    5,
			ExecType:    util.EXECRETRY,
			UpdateState: util.RETRY,
		},

		{
			TaskType:    util.C2,
			ErrOutline:  "timeout",
			RetryCnt:    5,
			ExecType:    util.EXECRETRY,
			UpdateState: util.RETRY,
		},

		{
			TaskType:    util.PRECOMMIT,
			ErrOutline:  "already been allocated (RetCode=16)",
			RetryCnt:    99,
			ExecType:    util.EXECNEXT,
			UpdateState: util.SUCCESS,
		},

		{
			TaskType:    util.PRECOMMIT,
			ErrOutline:  "already in mpool, increase GasPremium to",
			RetryCnt:    99,
			ExecType:    util.EXECRETRY,
			UpdateState: util.SUCCESS,
		},

		{
			TaskType:    util.PRECOMMIT,
			ErrOutline:  "message nonce too low",
			RetryCnt:    99,
			ExecType:    util.EXECRETRY,
			UpdateState: util.SUCCESS,
		},

		{
			TaskType:    util.SEED,
			ErrOutline:  "precommit info is not exists",
			RetryCnt:    3,
			ExecType:    util.EXECRETRY,
			UpdateState: util.SUCCESS,
		},

		{
			TaskType:    util.COMMIT,
			ErrOutline:  "reason: no pre-committed sector",
			RetryCnt:    5,
			ExecType:    util.EXECNEXT,
			UpdateState: util.RETRY,
		},

		{
			TaskType:    util.COMMIT,
			ErrOutline:  "message nonce too low",
			RetryCnt:    99,
			ExecType:    util.EXECRETRY,
			UpdateState: util.SUCCESS,
		},

		{
			TaskType:    util.COMMIT,
			ErrOutline:  "already in mpool, increase GasPremium to",
			RetryCnt:    99,
			ExecType:    util.EXECRETRY,
			UpdateState: util.SUCCESS,
		},
	}

	var err error
	tx := db.Begin()
	for _, tf := range taskFaileds {
		tf1 := tf
		err = tx.Where(tf1).FirstOrCreate(&tf1).Error
		if err != nil {
			log.Errorf("Init DbTaskFailed %v error %v", tf1, err)
			break
		}
	}
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}
