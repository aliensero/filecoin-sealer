# miner添加32GiB任务  
curl -X POST \    
	 -H "Content-Type: application/json" \    
	 --data '{ "jsonrpc": "2.0", "method": "NSMINER.AddTask", "params": [1012,1,"PC1","/root/miner_storage/t01012","/var/tmp/filecoin-proof-parameters/unsealed-34359738368","32GiB",8,"baga6ea4seaqao7s73y24kcutaosvacpdjgfe5pw76ooefnyqw4ynr3d2y6x2mpq"], "id": 1 }' \  
	 'http://127.0.0.1:4321/rpc/v0'  
  
# miner添加512MiB任务  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSMINER.AddTask", "params": [1013,1,"PC1","/root/miner_storage/t01013","/var/tmp/filecoin-proof-parameters/unsealed-536870912","512MiB",-1,""], "id": 1 }' \  
	 'http://127.0.0.1:4321/rpc/v0'  
  
# miner提交commit消息，通过lotus节点钱包签名  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSMINER.SendPreCommit", "params": [1012,-1,"0.05","0.025"], "id": 1 }' \  
	 'http://127.0.0.1:4321/rpc/v0'  
  
# miner提交precommit消息，通过私钥签名  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSMINER.SendPreCommitByPrivatKey", "params": ["7b2254797065223a22626c73222c22507269766174654b6579223a225a573273766c6a424d596447306179304b4f41567039576d54384f6f6b696e617a3964594e65346d4c7a553d227d",1018,33040,"1"], "id": 1 }' \  
	 'http://127.0.0.1:4321/rpc/v0'  
  
# miner提交commit消息，通过lotus节点钱包签名  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSMINER.SendCommit", "params": [1012,-1,"0.05","0.025"], "id": 1 }' \  
	 'http://127.0.0.1:4321/rpc/v0'  
  
# miner提交commit消息，通过私钥签名  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSMINER.SendCommitByPrivatKey", "params": ["7b2254797065223a22626c73222c22507269766174654b6579223a225a573273766c6a424d596447306179304b4f41567039576d54384f6f6b696e617a3964594e65346d4c7a553d227d",1018,33040,"1"], "id": 1 }' \  
	 'http://127.0.0.1:4321/rpc/v0'  
  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSMINER.WithDrawByPrivatKey", "params": ["7b2254797065223a22626c73222c22507269766174654b6579223a225a573273766c6a424d596447306179304b4f41567039576d54384f6f6b696e617a3964594e65346d4c7a553d227d",1018,"20",0], "id": 1 }' \  
	 'http://127.0.0.1:4321/rpc/v0'  
  
# miner创建矿工号  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSMINER.CreateMiner", "params": ["7b2254797065223a22626c73222c22507269766174654b6579223a224665577258514a48436762354c4d346d685733734f736e447278622b5a6d4670467a74496269754870426b3d227d","t3rybmrnmhcceqvq2434jeuzvmsjr7bgkmbjtm3ckyxrkwgfhew2w2zc7k4aw3jbmesba2cm6xh7d3zwxif6rq","t3rybmrnmhcceqvq2434jeuzvmsjr7bgkmbjtm3ckyxrkwgfhew2w2zc7k4aw3jbmesba2cm6xh7d3zwxif6rq",7], "id": 1 }' \  
	 'http://127.0.0.1:4321/rpc/v0'  
  
# miner获取seed随机数  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSMINER.GetSeedRand", "params": [1012,-1], "id": 1 }' \  
	 'http://127.0.0.1:4321/rpc/v0'  
  
# miner查看commit信息  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSMINER.StateSectorCommitInfo", "params": ["t01004",21939], "id": 1 }' \  
	 'http://127.0.0.1:4321/rpc/v0'  
  
# miner查看扇区precommit信息  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSMINER.StateSectorPreCommitInfo", "params": ["t01027",1], "id": 1 }' \  
	 'http://127.0.0.1:4321/rpc/v0'  
  
# miner获取ticket  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSMINER.GetTicket", "params": [1004,21940], "id": 1 }' \  
	 'http://127.0.0.1:4321/rpc/v0'  
  
# miner删除扇区文件  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSMINER.ClearProving", "params": [], "id": 1 }' \  
	 'http://127.0.0.1:4321/rpc/v0'  
  
# miner检查服务  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSMINER.CheckServer", "params": [],"id":1}'  'http://127.0.0.1:4321/rpc/v0'  
  
# miner检查任务  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSMINER.CheckSession", "params": ["cd0c8f38-0ea4-4c2c-abf6-77ee64d77d69"], "id": 1 }'  'http://127.0.0.1:4321/rpc/v0'  
  
# miner生成地址  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSMINER.GenerateAddress", "params": ["bls"], "id": 1 }'  'http://127.0.0.1:4321/rpc/v0'  
  
# 检查重置正在执行的任务  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSMINER.ResetAbortedSession", "params": [1034], "id": 1 }'  'http://127.0.0.1:4321/rpc/v0'  
  
# miner修复PoSt任务表  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSMINER.ReFindPoStTable", "params": [1034], "id": 1 }'  'http://127.0.0.1:4321/rpc/v0'  
  
# miner重置PoSt Map  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSMINER.ResetSyncPoStMap", "params": [], "id": 1 }'  'http://127.0.0.1:4321/rpc/v0'  
  
# miner 停止PoSt任务  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSMINER.AbortPoSt", "params": [1034], "id": 1 }'  'http://127.0.0.1:4321/rpc/v0'  
  
# miner 地址恢复PoSt  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSMINER.CheckRecoveries", "params": [1036,"t3rv4v233wts7vhi4cjltwe242jxlc6auluawha4yktyzoclfmtqtxz6i2q5dem74e2hers5xd4nmfrzy7plva","pub",0], "id": 1 }'  'http://127.0.0.1:4321/rpc/v0'  
  
# miner 私钥恢复PoSt  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSMINER.CheckRecoveries", "params": [1036,"7b2254797065223a22626c73222c22507269766174654b6579223a22503553576f4238614848336c3668665a7a7377775257734e662b765938372f6b4c727a69465254706e56513d227d","pri",0], "id": 1 }'  'http://127.0.0.1:4321/rpc/v0'  
  
  
  
  
  
# worker AddPiece  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.AddPiece", "params": ["32GiB","/tmp/32GiB",false], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
  
# worker读取piece信息  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.PieceInfo", "params": ["/var/tmp/filecoin-proof-parameters/pieceinfo-34359738368.txt"], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
	   
# worker启动pc1任务，新起协程  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.SealPreCommitPhase1", "params": [1012,-1], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
  
# worker启动pc1任务，新起进程  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.ProcessPrePhase1", "params": [1012,-1,"/usr/local/bin/worker"], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
  
# worker启动pc2任务，新起协程  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.SealPreCommitPhase2", "params": [1012,-1], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
  
# worker启动pc2任务，新起进程  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.ProcessPrePhase2", "params": [1012,-1,"/usr/local/bin/worker"], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
  
# worker启动c1任务，新起协程  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.SealCommitPhase1", "params": [1012,-1], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
  
# worker启动c1任务，新起进程  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.ProcessCommitPhase1", "params": [1012,-1,"/usr/local/bin/worker"], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
  
# worker启动c2任务，新起协程  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.SealCommitPhase2", "params": [1012,-1], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
  
# worker启动c2任务，新起进程  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.ProcessCommitPhase2", "params": [1012,-1,"/usr/local/bin/worker"], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'

# worker重启任务，新起进程 
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.RetryTaskPIDByState", "params": [1013,"PC1","/usr/local/bin/worker"], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'    
  
# worker强制重启任务，新起协程  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.RetryTask", "params": [1013,30027,"C1"], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
  
# worker强制重启任务，新起进程  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.RetryTaskPID", "params": [1034,200,"C1","/usr/local/bin/worker"], "id": 1 }' \  
	 'http://127.0.0.1:8742/rpc/v0'  
  
# worker修改任务限制  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.MotifyTaskLimit", "params": ["PC2",10], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
  
# worker修改任务超时  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.MotifyTaskTimeOut", "params": ["PC1",18000], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
  
# worker查看任务超时  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.DisplayTaskTimeOut", "params": [], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
  
# worker停止任务  
curl -X POST \  
	 -H "Content-Type: application/json" \  
	 --data '{ "jsonrpc": "2.0", "method": "NSWORKER.ShutdownReq", "params": ["f89fbce2-0beb-47b2-ab5a-1aa220718eea"], "id": 1 }' \  
	 'http://127.0.0.1:3456/rpc/v0'  
  
# worker删除扇区cache文件  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSWORKER.DeleteSectorCache", "params": [1018,40015], "id": 1 }'  'http://127.0.0.1:8745/rpc/v0'  
	   
# worker删除扇区文件  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSWORKER.DeleteSectorFile", "params": [1018,33235], "id": 1 }'  'http://127.0.0.1:8740/rpc/v0'  
  
# worker检查服务  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSWORKER.CheckServer", "params": [],"id":1}'  'http://127.0.0.1:8740/rpc/v0'  
  
# woker检查任务  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSWORKER.CheckSession", "params": ["7e9086d8-9540-490c-8ca2-2d81a1f59e8c"], "id": 1 }'  'http://127.0.0.1:8740/rpc/v0'  
  
# woker 私钥重试PoSt  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSWORKER.RetryWinPoSt", "params": [1036,"7b2254797065223a22626c73222c22507269766174654b6579223a22503553576f4238614848336c3668665a7a7377775257734e662b765938372f6b4c727a69465254706e56513d227d","pri"], "id": 1 }'  'http://127.0.0.1:8740/rpc/v0'  
  
# woker 地址重试PoSt  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSWORKER.RetryWinPoSt", "params": [1036,"t3rv4v233wts7vhi4cjltwe242jxlc6auluawha4yktyzoclfmtqtxz6i2q5dem74e2hers5xd4nmfrzy7plva","pub"], "id": 1 }'  'http://127.0.0.1:8740/rpc/v0'  
  
# woker 私钥添加PoSt  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSWORKER.AddPoStActor", "params": [1036,"7b2254797065223a22626c73222c22507269766174654b6579223a22503553576f4238614848336c3668665a7a7377775257734e662b765938372f6b4c727a69465254706e56513d227d","pri"], "id": 1 }'  'http://127.0.0.1:8740/rpc/v0'  
  
# woker 地址添加PoSt  
curl -X POST  -H "Content-Type: application/json"  --data '{ "jsonrpc": "2.0", "method": "NSWORKER.AddPoStActor", "params": [1036,"t3rv4v233wts7vhi4cjltwe242jxlc6auluawha4yktyzoclfmtqtxz6i2q5dem74e2hers5xd4nmfrzy7plva","pub"], "id": 1 }'  'http://127.0.0.1:8740/rpc/v0'  
  
  
  
# 耗时查询  
select actor_id,sector_num,task_type,state,created_at,updated_at,timediff(updated_at,created_at) as cost from db_task_logs where actor_id=1034 and state=2 and task_type='PC1' order by cost asc;  
  
# 任务查询  
select created_at,updated_at,actor_id,sector_num,task_type,state,worker_id,host_name from db_task_infos where actor_id=1034 and sector_num>199 order by updated_at asc;  
  
# mysql授权
GRANT ALL PRIVILEGES ON db_worker.* TO 'dockeruser'@'%' IDENTIFIED BY 'password';  

# 修改mysql消息打包大小  
show VARIABLES like '%max_allowed_packet%';    
set global max_allowed_packet = 104857600;    