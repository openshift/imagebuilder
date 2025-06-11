FROM mirror.gcr.io/debian
ADD check.sh main.sh /app/
CMD /app/main.sh
HEALTHCHECK
HEALTHCHECK --interval=5s --timeout=3s --retries=3 \
  CMD /app/check.sh --quiet
HEALTHCHECK CMD
HEALTHCHECK   CMD   a b
HEALTHCHECK --timeout=3s CMD ["foo"]
HEALTHCHECK --start-interval=5s --start-period=5s --interval=5s --timeout=3s --retries=3 \
  CMD /app/check.sh --quiet
HEALTHCHECK CONNECT TCP 7000
