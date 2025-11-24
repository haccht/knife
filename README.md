# knife

## Usage

`knife` reads text form stdin and display only columns you specify with flexible format.

Use `-F, --separator` to set the input field separators (default: whitespace).
Use `-j, --join` to change the separator used when rejoining selected fields (default: a single space).
Use `--buffer-size` to configure the buffered I/O size in bytes (default: 1MB) when processing very large inputs.

``` bash
$ cat sample.txt | knife <index>
```


## Example

Use `ps aux` as input text.

```bash
$ ps aux
USER         PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root           1  0.0  0.3 241300 13216 ?        Ss   Oct12   0:45 /lib/systemd/systemd --system --deserialize 42
root           2  0.0  0.0      0     0 ?        S    Oct12   0:00 [kthreadd]
root           3  0.0  0.0      0     0 ?        I<   Oct12   0:00 [rcu_gp]
root           4  0.0  0.0      0     0 ?        I<   Oct12   0:00 [rcu_par_gp]
root           5  0.0  0.0      0     0 ?        I<   Oct12   0:00 [slub_flushwq]
root           6  0.0  0.0      0     0 ?        I<   Oct12   0:00 [netns]
root           8  0.0  0.0      0     0 ?        I<   Oct12   0:00 [kworker/0:0H-events_highpri]
root          10  0.0  0.0      0     0 ?        I<   Oct12   0:00 [mm_percpu_wq]
root          11  0.0  0.0      0     0 ?        S    Oct12   0:00 [rcu_tasks_rude_]
```

Specify a single column:

```bash
$ ps aux | knife 2
PID
1
2
3
4
5
6
8
10
11
```

Specify multiple columns:

```bash
$ ps aux | knife 1 2
USER PID
root 1
root 2
root 3
root 4
root 5
root 6
root 8
root 10
root 11
```

Change the output separator with `-j` (e.g. create comma-separated output):

```bash
$ ps aux | knife 1 2 -j ,
USER,PID
root,1
root,2
root,3
root,4
root,5
root,6
root,8
root,10
root,11
```

Specify a single column from right with the negative index:

```bash
$ ps aux | knife -1
COMMAND
42
[kthreadd]
[rcu_gp]
[rcu_par_gp]
[slub_flushwq]
[netns]
[kworker/0:0H-events_highpri]
[mm_percpu_wq]
[rcu_tasks_rude_]
```

Specify multiple columns with the range format:

```bash
$ ps aux | knife 1:2 8:10
USER PID STAT START TIME
root 1 Ss Oct12 0:45
root 2 S Oct12 0:00
root 3 I< Oct12 0:00
root 4 I< Oct12 0:00
root 5 I< Oct12 0:00
root 6 I< Oct12 0:00
root 8 I< Oct12 0:00
root 10 I< Oct12 0:00
root 11 S Oct12 0:00
```

Specify multiple columns with the right open range format:

```bash
$ ps aux | knife 8:
STAT START TIME COMMAND
Ss Oct12 0:45 /lib/systemd/systemd --system --deserialize 42
S Oct12 0:00 [kthreadd]
I< Oct12 0:00 [rcu_gp]
I< Oct12 0:00 [rcu_par_gp]
I< Oct12 0:00 [slub_flushwq]
I< Oct12 0:00 [netns]
I< Oct12 0:00 [kworker/0:0H-events_highpri]
I< Oct12 0:00 [mm_percpu_wq]
S Oct12 0:00 [rcu_tasks_rude_]
```

Specify multiple columns with the left open range format:

```bash
$ ps aux | knife :4
USER PID %CPU %MEM
root 1 0.0 0.3
root 2 0.0 0.0
root 3 0.0 0.0
root 4 0.0 0.0
root 5 0.0 0.0
root 6 0.0 0.0
root 8 0.0 0.0
root 10 0.0 0.0
root 11 0.0 0.0
```

Specify multiple columns with the reverted range format:

```bash
$ ps aux | knife 2:1
PID USER
1 root
2 root
3 root
4 root
5 root
6 root
8 root
10 root
11 root
```

Reorder columns:

```bash
$ ps aux | knife 2:1 3:
PID USER %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND
1 root 0.0 0.3 241300 13216 ? Ss Oct12 0:45 /lib/systemd/systemd --system --deserialize 42
2 root 0.0 0.0 0 0 ? S Oct12 0:00 [kthreadd]
3 root 0.0 0.0 0 0 ? I< Oct12 0:00 [rcu_gp]
4 root 0.0 0.0 0 0 ? I< Oct12 0:00 [rcu_par_gp]
5 root 0.0 0.0 0 0 ? I< Oct12 0:00 [slub_flushwq]
6 root 0.0 0.0 0 0 ? I< Oct12 0:00 [netns]
8 root 0.0 0.0 0 0 ? I< Oct12 0:00 [kworker/0:0H-events_highpri]
10 root 0.0 0.0 0 0 ? I< Oct12 0:00 [mm_percpu_wq]
11 root 0.0 0.0 0 0 ? S Oct12 0:00 [rcu_tasks_rude_]
```

If you have to align columns:

```bash
$ ps aux | knife 1:8 | column -t
USER  PID  %CPU  %MEM  VSZ     RSS    TTY  STAT
root  1    0.0   0.3   241300  13216  ?    Ss
root  2    0.0   0.0   0       0      ?    S
root  3    0.0   0.0   0       0      ?    I<
root  4    0.0   0.0   0       0      ?    I<
root  5    0.0   0.0   0       0      ?    I<
root  6    0.0   0.0   0       0      ?    I<
root  8    0.0   0.0   0       0      ?    I<
root  10   0.0   0.0   0       0      ?    I<
root  11   0.0   0.0   0       0      ?    S
```

## Performance

`awk` and `cut` commands are still faster...

`knife` uses 1MB buffered I/O for both reading and writing by default (configurable via `--buffer-size`) and defers flushes until the end of the stream. This reduces syscall overhead and helps throughput when processing large datasets.

```bash
$ time ( cat large_text.txt | knife 1:3 | wc -l )
1000000

real    0m1.486s
user    0m1.021s
sys     0m1.092s


$ time ( cat large_text.txt | awk '{print $1,$2,$3}' | wc -l )
1000000

real	0m0.579s
user	0m0.515s
sys	    0m0.139s


$ time ( cat large_text.txt | tr -s ' ' | cut -d ' ' -f 1,2,3 | wc -l )
1000000

real    0m0.749s
user    0m0.749s
sys     0m0.568s
```
