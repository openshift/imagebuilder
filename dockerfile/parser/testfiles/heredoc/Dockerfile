FROM docker.io/library/python:latest
RUN <<EOF
echo "Hello" >> /hello
echo "World!" >> /hello
EOF

RUN <<-EOF
echo "hello"
echo "world"
EOF

RUN <<heredoc
echo "hello"
echo "world"
heredoc

RUN <<-heredoc
echo "hello"
echo "world"
heredoc

RUN 0<<heredoc
echo "hello"
echo "world"
heredoc

RUN 0<<-heredoc
echo "hello"
echo "world"
heredoc

RUN 0<<-"heredoc"
echo "hello"
echo "world"
heredoc

RUN 0<<"heredoc"
echo "hello"
echo "world"
heredoc

RUN 5<<EOF cat /proc/self/fd/5 > file.txt
this is the file
EOF

RUN cat /proc/self/fd/5 /proc/self/fd/6 5<<FILE1 6<<FILE2 > file.txt
this is the first file
FILE1
this is the second file
FILE2

RUN cat /proc/self/fd/5 /proc/self/fd/6 5<<FILE1 > file.txt 6<<FILE2
this is the first file
FILE1
this is the second file
FILE2

RUN cat 0<<EOF > file.txt
this is the file
EOF

RUN 5<<file cat /proc/self/fd/5 /proc/self/fd/6 6<<FILE | cat /dev/stdin /proc/self/fd/6 6<<File > file.txt
this is the first file
file
this is the second file
FILE
this is the third file
File

RUN python3 <<EOF
with open("/hello", "w") as f:
    print("Hello", file=f)
    print("Something", file=f)
EOF

ADD <<EOF /index.html
(your index page goes here)
EOF

COPY <<robots.txt <<humans.txt /test/
(robots content)
robots.txt
(humans content)
humans.txt

# heredoc with terminator followed by
# non-empty line.
RUN 0<<-"heredoc"
echo "hello"
echo "world"
heredoc
RUN cat index.html
RUN cat hello
