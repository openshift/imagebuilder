FROM public.ecr.aws/docker/library/centos:7
COPY --chown=1:2     script /usr/bin/script.12
COPY --chown=1:adm   script /usr/bin/script.1-adm
COPY --chown=1       script /usr/bin/script.1
COPY --chown=lp:adm  script /usr/bin/script.lp-adm
COPY --chown=2:mail  script /usr/bin/script.2-mail
COPY --chown=2       script /usr/bin/script.2
COPY --chown=bin     script /usr/bin/script.bin
COPY --chown=lp      script /usr/bin/script.lp
COPY --chown=3       script script2 /usr/local/bin/
RUN  rm -fr /var/created-directory
COPY --chown=12345 script script2 /var/created/directory/
RUN  rm -fr /no-such-directory
COPY --chown=3       script script2 /no-such-directory/
RUN  rm -fr /new-workdir
WORKDIR /new-workdir/several/levels/deep
COPY --chown=3       script script2 no-such-directory/
WORKDIR ../deeper
COPY --chown=3       script script2 no-such-directory-either/
COPY --chown=3       script script2 ../no-such-subdirectory/
