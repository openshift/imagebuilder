ARG ALPINE

FROM mirror.gcr.io/busybox
RUN echo TARGETPLATFORM=$TARGETPLATFORM | tee 0.txt
ARG TARGETPLATFORM
RUN echo TARGETPLATFORM=$TARGETPLATFORM | tee -a 0.txt
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN echo TARGETOS=$TARGETOS TARGETARCH=$TARGETARCH TARGETVARIANT=$TARGETVARIANT | tee -a 0.txt
RUN echo BUILDPLATFORM=$BUILDPLATFORM | tee 0.txt
ARG BUILDPLATFORM
RUN echo BUILDPLATFORM=$BUILDPLATFORM | tee -a 0.txt
ARG BUILDOS
ARG BUILDARCH
ARG BUILDVARIANT
RUN echo BUILDOS=$BUILDOS BUILDARCH=$BUILDARCH BUILDVARIANT=$BUILDVARIANT | tee -a 0.txt
RUN touch -d @0 0.txt

FROM ${SECONDBASE:-mirror.gcr.io/busybox}
COPY --from=0 /*.txt /
COPY --chown=${OWNERID:-1}:${OWNERID:-1} ${SOURCE:-other}file.txt /1a.txt
ARG OWNERID=1
ARG SOURCE=
COPY --chown=${OWNERID:-1}:${OWNERID:-1} ${SOURCE:-other}file.txt /1b.txt

FROM ${ALPINE:-mirror.gcr.io/busybox}
ARG SECONDBASE=localhost/no-such-image
COPY --from=1 /*.txt /
RUN cp /etc/nsswitch.conf /2.txt

FROM ${BUSYBOX:-mirror.gcr.io/alpine}
COPY --from=2 /*.txt /
RUN cp /etc/nsswitch.conf /3.txt
