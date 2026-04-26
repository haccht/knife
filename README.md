# knife

## Usage

`knife` reads text form stdin and display only columns you specify with flexible format.


``` bash
$ knife -h
Usage:
  knife [OPTIONS]

Application Options:
  -F, --field-separator= Single-byte field separator. Repeat to use multiple separators.
      --buffer-size=     Buffer size in bytes for buffered I/O (default: 1MB)

Help Options:
  -h, --help             Show this help message


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

Explicit delimiters preserve empty fields:

```bash
$ printf 'a,,,c\n' | knife -F, 1:4
a   c
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

Extract the first regexp match from selected columns. This is useful when a field contains a tagged value and you only need the value part. If a field does not match, the original field will be printed:

```bash
$ cat <<'EOF' | knife '2@[^=]+$' '3@[^=]+$' '4@[0-9]+'
2026-04-01T10:00:00Z user=alice request_id=req-42 status=200
2026-04-01T10:00:01Z user=bob request_id=req-77 status=500
EOF
alice req-42 200
bob req-77 500
```

Apply a regexp to every field selected by a range. For example, extract numeric metric values from several tagged fields:

```bash
$ cat <<'EOF' | knife '2:4@[0-9.]+'
api latency=12.8ms size=1536B retry=2
web latency=8.4ms size=512B retry=0
EOF
12.8 1536 2
8.4 512 0
```

Pipe selected fields to a shell command and replace them with the command output. This is useful for conversions that are easier to express with an existing tool:

```bash
$ cat <<'EOF' | knife '4|numfmt --to=iec'
GET /a 204 1536
POST /upload 201 1048576
EOF
1.5K
1.0M
```

The command is started once per selector command, not once per input line. While reading input, `knife` sends each selected field to the command's standard input as one line, and uses one line of standard output as the replacement for each field. Commands used this way must emit exactly one output line for each selected field.

You can combine regexp extraction and command replacement:

```bash
$ cat <<'EOF' | knife '2@[^=]+$|tr A-Z a-z'
alice email=ALICE@EXAMPLE.COM
bob email=BOB@EXAMPLE.NET
EOF
alice@example.com
bob@example.net
```

## Performance

`knife` uses 1MB buffered I/O for both reading and writing by default (configurable via `--buffer-size`) and defers flushes until the end of the stream. This reduces syscall overhead and helps throughput when processing large datasets.
`knife` is faster than `awk` and `cut`.


```bash
$ time ( cat large_file.txt | knife 1 2 3 | wc -l )
1034472

real    0m0.497s
user    0m0.406s
sys     0m0.188s


$ time ( cat large_file.txt | awk '{print $1,$2,$3}' | wc -l )
1034472

real    0m0.749s
user    0m0.703s
sys     0m0.313s


$ time ( cat large_file.txt | tr -s ' ' | cut -d ' ' -f 1,2,3 | wc -l )
1034472

real    0m0.594s
user    0m0.906s
sys     0m0.359s
```
