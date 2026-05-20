FROM python:3.12-alpine

RUN addgroup -S -g 65532 sandbox && adduser -S -D -H -u 65532 -G sandbox sandbox

COPY runner.py /runner.py

USER 65532:65532
ENTRYPOINT ["python3", "/runner.py"]
