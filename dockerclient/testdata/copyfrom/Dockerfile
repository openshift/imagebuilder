FROM centos:7 as base
RUN mkdir -p /a/blah && touch /a/blah/1 /a/blah/2
RUN mkdir -m 711 /711 && touch /711/711.txt
RUN mkdir -m 755 /755 && touch /755/755.txt
RUN mkdir -m 777 /777 && touch /777/777.txt
FROM centos:7
COPY --from=base /a/blah/* /blah/
RUN rm -fr /711 /755 /777
COPY --from=0 /711 /711
COPY --from=0 /755 /755
COPY --from=0 /777 /777
RUN mkdir /precreated /precreated/711 /precreated/755 /precreated/777
COPY --from=0 /711 /precreated/711
COPY --from=0 /755 /precreated/755
COPY --from=0 /777 /precreated/777
