FROM busybox
RUN rm -fr /var/lib/not-in-this-image
VOLUME /var/lib/not-in-this-image
RUN mkdir -p /var/lib
RUN touch /var/lib/file-not-in-image
