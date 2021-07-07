# ns-miner命令
### 启动服务

    ns-miner daemon

    参数：

        --listien           服务监听
        --user              mysql用户
        --passwd            mysql密码
        --ip                myql IP
        --port              mysql端口
        --name              mysql数据库名
        --lotus-addr        lotus api
        --lotus-token       lotus token，可以为空，线下签名
        --waitseedecpoch    等待seed多少个高度
        --expiration        扇区生命周期高度

    例如：ns-miner daemon --lotus-addr https://filestar.info/rpc/v0 --lotus-token ""

### 自动任务，precommit,seed,commit
 
    ns-miner taskauto minerID 私钥(为空，命令行隐藏输入)

    参数：
        
        --add           添加自动任务
        --del           清空自动任务
        --dis           显示正在进行的自动任务
        --minerapi      ns-miner daemon服务启动监听的api，可以取
                        $MINERAPI环境变量

    例如：ns-miner taskauto 10001 ""

### 手工触发任务

    ns-miner tasktrig minerID 扇区编号 任务类型[PRECOMMIT SEED COMMIT]

    参数：

        --prikey        worker私钥,为空时,命令行输入
        --deposit       转账金额
        --minerapi      ns-miner daemon服务启动监听的api，可以取
                        $MINERAPI环境变量
    
### 添加封装任务
    
    ns-miner addtask

    参数：

        --storagePath   扇区数据保存位置，环境变量$STORAGE_PATH
        --unsealPath    垃圾数据原始文件位置，环境变量$UNSEAL_PATH
        --sealerProof   扇区大小
        --proofType     扇区类型
        --pieceCID      垃圾数据CID
        --minerapi      ns-miner daemon服务启动监听的api，可以取
                        $MINERAPI环境变量

### 创建矿工
    
    ns-miner createminer owner地址 worker地址

    参数：

        --prikey        worker私钥，为空时，命令行输入
        --type          miner封装扇区类型
        --minerapi      ns-miner daemon服务启动监听的api，可以取
                        $MINERAPI环境变量

### 转账

    ns-miner send 转入地址 转账金额

    参数：

        --prikey        转出地址私钥，为空时，命令行输入
        --minerapi      ns-miner daemon服务启动监听的api，可以取
                        $MINERAPI环境变量

### 创建地址

    ns-miner newaddr

    描述：保存在表db_address_infos

    参数：

        --type          地址类型
        --minerapi      ns-miner daemon服务启动监听的api，可以取
                        $MINERAPI环境变量

### 替换消息手续费

    ns-miner replace 消息cid cap值 premium值 limit值

    参数：

        --prikey        私钥，为空时，命令行输入
        --minerapi      ns-miner daemon服务启动监听的api，可以取
                        $MINERAPI环境变量

### 恢复window PoSt失败的扇区

    ns-miner recover minerID 私钥(为空，命令行输入) deadline编号

    参数：

        --minerapi      ns-miner daemon服务启动监听的api，可以取
                        $MINERAPI环境变量

    例如：ns-miner recover 1001 "" 0

### 添加window PoSt任务

    ns-miner addPost miner地址 私钥(为空时，命令行输入)

    参数：

        --minerapi      ns-miner daemon服务启动监听的api，可以取
                        $MINERAPI环境变量

    例如：ns-miner addPost f01001 ""

### 重试window PoSt任务

    ns-miner retryPost minerID 私钥(为空时，命令行输入)

    参数：

        --minerapi      ns-miner daemon服务启动监听的api，可以取
                        $MINERAPI环境变量

    例如：ns-miner retryPost 1001 ""

### 手动执行window PoSt任务(实验功能)

    ns-miner manualPost minerID 私钥(为空时，命令行输入)

    描述：可以根据提供的扇区文件信息进行PoSt计算，文件内容格式如下：
          扇区编号 扇区comm_r

    参数：

        --submit        是否提交PoSt消息上链
        --ptype         扇区PoSt类型
        --cepoch        PoSt挑战高度
        --sipaht        扇区信息文件路径
        --spath         扇区文件路径,可取环境变量$STORAGE_PATH
        --dindex        deadline编号
        --pindex        partition编号
        --minerapi      ns-miner daemon服务启动监听的api，可以取
                        $MINERAPI环境变量

    例如：ns-miner manualPost f01001 ""

### 挖矿出块服务

    ns-miner mining 扇区文件多路径

    参数：
    
        --privatekey        私钥，为空时，命令行输入
        --actorid           minerID
        --lotusapi          公开lotus api,可取环境变量$LOTUS_API
        --netname           p2p网络名

    例如：ns-miner mining /storage1 /storage2


# ns-worker命令
### worker服务启动

    ns-worker run
    
    参数：
        
        --listen        worker服务监听地址
        --taskType      worker可执行的任务类型
        --minerapi      miner api
        --pc1lmt        限制并行pc1任务数量
        --pc2lmt        限制并行pc2任务数量
        --c1lmt         限制并行c1任务数量
        --c2lmt         限制并行c2任务数量
        --pc1to         设置pc1任务超时时间
        --pc2to         设置pc2任务超时时间
        --c1to          设置c1任务超时时间
        --c2to          设置c2任务超时时间
        --workerid      worker ID

### worker触发任务

    ns-worker tasktrig actorID 扇区编号 任务类型

    参数：
        
        --bin           worker可执行二进制路径，可取环境变量$WORKER_BIN_PATH
        --restart       是否为重启
        --workerapi     worker api，可取环境变量$WORKER_API

### worker自动任务

    ns-worker taskauto  [PC1 PC2 C1 C2]

    参数:

        --actorid       miner ID，如：10001
        --add           添加自动任务类型
        --del           清除所有自动任务
        --dis           显示自动任务
        --workerapi     worker api，可取环境变量$WORKER_API
