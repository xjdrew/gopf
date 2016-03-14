## make
go install gopf

## benchmark
* use nginx as backend
* export GOMAXPROCS=1 && go install gopf && bin/gopf settings.conf

* use gopf as proxy: ab -n 10000 -c 100 http://127.0.0.1:1248/
```
Server Software:        nginx/1.2.1
Server Hostname:        127.0.0.1
Server Port:            1248

Document Path:          /
Document Length:        396 bytes

Concurrency Level:      100
Time taken for tests:   0.816 seconds
Complete requests:      10000
Failed requests:        0
Write errors:           0
Total transferred:      6060000 bytes
HTML transferred:       3960000 bytes
Requests per second:    12256.13 [#/sec] (mean)
Time per request:       8.159 [ms] (mean)
Time per request:       0.082 [ms] (mean, across all concurrent requests)
Transfer rate:          7253.14 [Kbytes/sec] received

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        0    0   0.4      0       4
Processing:     3    8   1.3      8      18
Waiting:        2    6   1.4      6      14
Total:          3    8   1.4      8      18
```

* connnect nginx direct: ab -n 10000 -c 100 http://127.0.0.1/
```
Server Software:        nginx/1.2.1
Server Hostname:        127.0.0.1
Server Port:            80

Document Path:          /
Document Length:        396 bytes

Concurrency Level:      100
Time taken for tests:   0.477 seconds
Complete requests:      10000
Failed requests:        0
Write errors:           0
Total transferred:      6060000 bytes
HTML transferred:       3960000 bytes
Requests per second:    20945.74 [#/sec] (mean)
Time per request:       4.774 [ms] (mean)
Time per request:       0.048 [ms] (mean, across all concurrent requests)
Transfer rate:          12395.62 [Kbytes/sec] received

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        1    2   0.3      2       4
Processing:     1    3   0.5      2       8
Waiting:        1    2   0.5      2       5
Total:          3    5   0.5      5      11
WARNING: The median and mean for the processing time are not within a normal deviation
        These results are probably not that reliable.
```

* much faster than pen(http://siag.nu/pen/)
