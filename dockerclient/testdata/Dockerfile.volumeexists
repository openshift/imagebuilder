FROM busybox
RUN mkdir -p 0700 /var/lib/bespoke-directory
RUN chown 1:1 /var/lib/bespoke-directory
VOLUME /var/lib/bespoke-directory
RUN touch /var/lib/bespoke-directory/emptyfile
