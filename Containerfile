
FROM ubi8/go-toolset:1.18 as build

### Copy source code for building the application
COPY . .

### Download dependencies and build
RUN go mod init gomandel && \
    go mod tidy -e && \
    go build . 

FROM ubi8/ubi-micro
COPY --from=build /opt/app-root/src/gomandel .

EXPOSE 8080
ENTRYPOINT ["./gomandel", "--server"]
CMD [ "--xres=1500", "--yres=900" ]
