FROM rust:1.31

COPY service /opt/my/service

WORKDIR /opt/my/service

RUN rustc --version && \
	cargo build --release && \
	cp target/release/rust-service .

EXPOSE 15000
ENTRYPOINT ["/opt/my/service/rust-service"]
