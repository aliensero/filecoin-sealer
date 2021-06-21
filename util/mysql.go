package util

import (
	"fmt"
	"os"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"

	logging "github.com/ipfs/go-log/v2"
)

func init() {
	_ = logging.SetLogLevel("mysql-struct", "DEBUG")
}

var log = logging.Logger("mysql-struct")

const (
	PC1       = "PC1"
	PC2       = "PC2"
	PRECOMMIT = "PRECOMMIT"
	SEED      = "SEED"
	C1        = "C1"
	C2        = "C2"
	COMMIT    = "COMMIT"
	PROVING   = "PROVING"
	INIT      = 0
	RUNING    = 1
	SUCCESS   = 2
	ERROR     = 3
	RETRY     = 4
	EXECRETRY = "retry"
	EXECNEXT  = "next"
)

var NextTask = map[string]string{
	PC1:       PC2,
	PC2:       PRECOMMIT,
	PRECOMMIT: SEED,
	SEED:      C1,
	C1:        C2,
	C2:        COMMIT,
	COMMIT:    PROVING,
}

type Model struct {
	ID        int64 `gorm:"primary_key;AUTO_INCREMENT"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ModelNoKey struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

type DbWorkerRegister struct {
	Model
	HostName       string `gorm:"index;unique"`
	Ip             string
	Port           string
	TaskType       string `gorm:"default:'ALL'"`
	CurrentTaskCnt int64  `gorm:"default:0"`
	MaxTaskCnt     int64  `gorm:"default:5"`
}

type RequestInfo struct {
	ActorID      int64
	SectorNum    int64
	TaskType     string
	Session      string
	WorkerID     string
	HostName     string
	WorkerListen string
	Sectors      string
}

type DbTaskInfo struct {
	Model
	SectorNum *int64 `gorm:"primary_key;AUTO_INCREMENT:false;default:0"`
	ActorID   *int64 `gorm:"primary_key;AUTO_INCREMENT:false;default:0"`
	TaskType  string `gorm:"primary_key"`

	SealerProof string `gorm:"default:'512MiB'"`
	ProofType   int64  `gorm:"default:7"`
	PieceStr    string `gorm:"default:'baga6ea4seaqdsvqopmj2soyhujb72jza76t4wpq5fzifvm3ctz47iyytkewnubq'"`

	CacheDirPath     string
	StagedSectorPath string `gorm:"default:'/tmp/512MiB'"`
	SealedSectorPath string

	WorkerID     string
	HostName     string
	WorkerListen string

	Phase1Output []byte `gorm:"type:mediumblob"`

	CommD string
	CommR string

	LastReqID string

	TicketEpoch int64
	TicketHex   string

	SeedEpoch int64
	SeedHex   string

	C1Out []byte `gorm:"type:longblob"`

	Proof []byte `gorm:"type:mediumblob"`

	State *int64 `gorm:"default:0"`

	DeadlineInx  uint64
	PartitionInx uint64
}

type DbTaskLog struct {
	Model

	SectorNum int64  `gorm:"primary_key;AUTO_INCREMENT:false;default:0"`
	ActorID   int64  `gorm:"primary_key;AUTO_INCREMENT:false;default:0"`
	TaskType  string `gorm:"primary_key"`

	ReqID string `gorm:"primary_key"`

	WorkerID     string
	HostName     string
	WorkerListen string

	State  int64  `gorm:"index;default:0"`
	Result string `gorm:"type:Text"`
}

type DbPostInfo struct {
	Model

	ActorID          int64
	SectorNum        int64
	WorkerID         string
	CommR            string
	CacheDirPath     string
	SealedSectorPath string
	ProofType        int64
	DeadlineInx      uint64
	PartitionInx     uint64
	State            int64
}

type DbAddressInfo struct {
	Model
	PrivateKey string
	Address    string `gorm:"primary_key"`
}

type DbTaskFailed struct {
	Model
	TaskType    string
	ExecType    string `gorg:"default:'retry'"`
	ErrOutline  string
	UpdateState int64
	RetryCnt    int64 `gorm:"default:5"`
}

type DbWorkerLogin struct {
	Model
	WorkerID string
	HostName string
	Listen   string
	State    int64
}

const (
	DR = "DR"
	CR = "CR"
)

type DbTransaction struct {
	Model
	Cid     string
	Acct    string
	Type    string
	Value   string
	Method  uint64
	NetName string
}

func InitMysql(user, passwd, ip, port, database string) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, passwd, ip, port, database)
	db, err := gorm.Open("mysql", dsn)

	if err != nil {
		log.Errorf("init mysql error %v\n", err)
		return nil, err
	} else {
		sqlDB := db.DB()
		sqlDB.SetMaxIdleConns(10)  //空闲连接数
		sqlDB.SetMaxOpenConns(100) //最大连接数
		sqlDB.SetConnMaxLifetime(time.Minute)
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&DbTaskLog{}, &DbTaskInfo{}, &DbTaskFailed{}, &DbAddressInfo{}, &DbWorkerLogin{}, &DbPostInfo{}, &DbTransaction{})
		// w.db.Model(&DbWorkerRegister{}).AddIndex("idx_host_name", "host_name")
	}
	if _, ok := os.LookupEnv("MYSQLDEBUG"); ok {
		db = db.Debug()
	}
	return db, err
}
