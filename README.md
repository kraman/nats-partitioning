This is an attempt to replicate the functionality that Kafka provides with queue groups in a Golang/CSP style.

* Uses Bully algorithm over NATS to elect leader
* Uses NATS Streaming to maintain partiton ownership
* Leader updates partition owners based on alive members
* Member assigned a partition by leader processes messages from associated NATS streaming queue

NOTE: this is prototype code. Not even close to production ready.

```sh
> make start-stan
> POD_IP=127.0.0.1 go run main.go
> POD_IP=127.0.0.2 go run main.go
> POD_IP=127.0.0.3 go run main.go
```

---

```
artemis:nats-test kraman$ POD_IP=127.0.0.1 go run -race main.go
DEBU[0000] post member event member-join Leader: false, Addr: 127.0.0.1, Tags: map[], Status: alive, ID: 0d166488-d243-490c-9e63-336d84cb5fc0 
[10444] 2019/08/25 10:54:41.771508 [INF] STREAM: Channel "_PARTITIONS.test" has been created
[10444] 2019/08/25 10:54:42.274156 [INF] STREAM: Channel "_PARTITIONS.test.3" has been created
DEBU[0001] starting election                            
DEBU[0001] stopping election on this node               
DEBU[0003] election self as leader                      
DEBU[0003] stopping election on this node               
DEBU[0003] post member event member-is-leader Leader: true, Addr: 127.0.0.1, Tags: map[], Status: alive, ID: 0d166488-d243-490c-9e63-336d84cb5fc0 
DEBU[0003] publishing assignment update                 
INFO[0003] reserve partition 4                          
INFO[0003] reserve partition 1                          
INFO[0003] reserve partition 2                          
INFO[0003] reserve partition 7                          
INFO[0003] reserve partition 9                          
INFO[0003] reserve partition 3                          
INFO[0003] reserve partition 5                          
INFO[0003] reserve partition 6                          
INFO[0003] reserve partition 0                          
INFO[0003] reserve partition 8                          
[10444] 2019/08/25 10:54:44.786417 [INF] STREAM: Channel "_PARTITIONS.test.4" has been created
[10444] 2019/08/25 10:54:44.787507 [INF] STREAM: Channel "_PARTITIONS.test.1" has been created
[10444] 2019/08/25 10:54:44.788749 [INF] STREAM: Channel "_PARTITIONS.test.2" has been created
[10444] 2019/08/25 10:54:44.789993 [INF] STREAM: Channel "_PARTITIONS.test.7" has been created
[10444] 2019/08/25 10:54:44.791084 [INF] STREAM: Channel "_PARTITIONS.test.9" has been created
[10444] 2019/08/25 10:54:44.793626 [INF] STREAM: Channel "_PARTITIONS.test.5" has been created
[10444] 2019/08/25 10:54:44.794677 [INF] STREAM: Channel "_PARTITIONS.test.6" has been created
[10444] 2019/08/25 10:54:44.795585 [INF] STREAM: Channel "_PARTITIONS.test.0" has been created
[10444] 2019/08/25 10:54:44.796470 [INF] STREAM: Channel "_PARTITIONS.test.8" has been created
ack sequence:6 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.5" timestamp:1566705284772205000 
ack sequence:7 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.6" timestamp:1566705285275044000 
ack sequence:8 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.7" timestamp:1566705285775814000 
ack sequence:9 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.8" timestamp:1566705286274505000 
... 
ack sequence:17 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.16" timestamp:1566705290272268000 
ack sequence:18 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.17" timestamp:1566705290771857000 
ack sequence:19 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.18" timestamp:1566705291272366000 
ack sequence:20 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.19" timestamp:1566705291771863000 
ack sequence:21 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.20" timestamp:1566705292276321000 
ack sequence:22 subject:"_PARTITIONS.test.3" data:"e1aa5263-7441-4392-b3de-9e2f4a8890dc.0" timestamp:1566705292699575000 
DEBU[0010] post member event member-join Leader: false, Addr: 127.0.0.2, Tags: map[], Status: alive, ID: e1aa5263-7441-4392-b3de-9e2f4a8890dc 
DEBU[0010] publishing assignment update                 
INFO[0010] enqueue 1 for release (e1aa5263-7441-4392-b3de-9e2f4a8890dc, 0d166488-d243-490c-9e63-336d84cb5fc0) 
INFO[0010] enqueue 7 for release (e1aa5263-7441-4392-b3de-9e2f4a8890dc, 0d166488-d243-490c-9e63-336d84cb5fc0) 
INFO[0010] enqueue 3 for release (e1aa5263-7441-4392-b3de-9e2f4a8890dc, 0d166488-d243-490c-9e63-336d84cb5fc0) 
INFO[0010] enqueue 5 for release (e1aa5263-7441-4392-b3de-9e2f4a8890dc, 0d166488-d243-490c-9e63-336d84cb5fc0) 
INFO[0010] enqueue 9 for release (e1aa5263-7441-4392-b3de-9e2f4a8890dc, 0d166488-d243-490c-9e63-336d84cb5fc0) 
DEBU[0010] releasing partition 1                        
DEBU[0010] releasing partition 7                        
DEBU[0010] releasing partition 3                        
DEBU[0010] releasing partition 5                        
DEBU[0010] releasing partition 9                        
```

```
artemis:nats-test kraman$ POD_IP=127.0.0.2 go run main.go
DEBU[0000] post member event member-join Leader: false, Addr: 127.0.0.2, Tags: map[], Status: alive, ID: e1aa5263-7441-4392-b3de-9e2f4a8890dc 
DEBU[0000] post member event member-join Leader: true, Addr: 127.0.0.1, Tags: map[], Status: alive, ID: 0d166488-d243-490c-9e63-336d84cb5fc0 
INFO[0000] wait for other node to release partition 9   
INFO[0000] wait for other node to release partition 1   
INFO[0000] wait for other node to release partition 5   
INFO[0000] wait for other node to release partition 7   
INFO[0000] wait for other node to release partition 3   
INFO[0000] reserve partition 1                          
INFO[0000] reserve partition 7                          
INFO[0000] reserve partition 3                          
INFO[0000] reserve partition 5                          
ack sequence:22 subject:"_PARTITIONS.test.3" data:"e1aa5263-7441-4392-b3de-9e2f4a8890dc.0" timestamp:1566705292699575000 
INFO[0000] reserve partition 9                          
ack sequence:23 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.21" timestamp:1566705292774906000 
ack sequence:24 subject:"_PARTITIONS.test.3" data:"e1aa5263-7441-4392-b3de-9e2f4a8890dc.1" timestamp:1566705293200307000 
ack sequence:25 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.22" timestamp:1566705293273911000 
ack sequence:26 subject:"_PARTITIONS.test.3" data:"e1aa5263-7441-4392-b3de-9e2f4a8890dc.2" timestamp:1566705293700714000 
ack sequence:27 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.23" timestamp:1566705293772647000 
...
ack sequence:39 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.29" timestamp:1566705296774374000 
ack sequence:40 subject:"_PARTITIONS.test.3" data:"e1aa5263-7441-4392-b3de-9e2f4a8890dc.9" timestamp:1566705297200519000 
ack sequence:41 subject:"_PARTITIONS.test.3" data:"0d166488-d243-490c-9e63-336d84cb5fc0.30" timestamp:1566705297277261000 
ack sequence:42 subject:"_PARTITIONS.test.3" data:"e1aa5263-7441-4392-b3de-9e2f4a8890dc.10" timestamp:1566705297700769000 
```