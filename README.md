# GRPC-WEB-PROXY-GW

A proxy for the Hyperledger Fabric gateway service. So, what's this for? Well, for connecting a web client to Fabric, for example.

## But what does it _actually_ do?

Want to make a web app that send transactions directly to Fabric? You need this.

This proxy is the mediator between Fabric and the browser. It translates gRPC-Web requests from the browser into native gRPC that Fabric understands, and vice versa. It even has event streaming using WebSockets!!

## What you'll need

- **Go 1.21+**. Because we use `slog`, the standard library's logger.
- **A Fabric network**. But it must have the Gateway service enabled on at least one peer.
- **Fabric Credentials**. You need at least one certificate and a private key from a registered user so the proxy can identify itself. Basically, it needs a digital identity to connect.

## How it works

1.  Clone the repository.
2.  Open a terminal.

```bash
go build -o fabric-gw-proxy ./cmd/proxy/
```

3.  This project also needs a configuration file. Here's an example of the `config.yaml` you need to have in the root of the project so it gets picked up.

```yaml
#~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~#
#               Our Proxy's Survival Guide                  #
#~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~#
# Fill this out to tell our brave proxy how to behave in the
# wild world of the network.

server:
  # Where our proxy is going to set up shop.
  # Format: "host:port"
  listenAddr: "localhost:8088"

  # When we tell it to shut down, how many seconds do we politely
  # wait for connections to finish before pulling the plug?
  shutdownTimeout: 5

  # The VIP list. If a request's origin isn't on this list, it's not getting in.
  # "*" is like an open-door party, but in production, you'll want
  # to be a bit more selective.
  # This works for both CORS (gRPC-Web) and WebSockets.
  allowedOrigins:
    - "http://localhost:3000"
    - "http://127.0.0.1:3000"
    - "https://our-awesome-app.com"

fabric:
  # The buddy we call by default.
  # When a request arrives not knowing which Fabric peer to talk to,
  # the proxy sends it here.
  gatewayAddress: "peer0.org1.example.com:7051"

  # The name we use to verify the peer's TLS certificate.
  # It usually matches the gateway address, but sometimes life is complicated.
  hostname: "peer0.org1.example.com"

  tls:
    # The "live on the edge" switch.
    # Set it to 'true' for production, unless you like danger.
    enabled: true

    # The Holy Trinity of mTLS security.
    # If 'enabled' is true, these three paths must lead somewhere.

    # The network's passport: the TLS Certificate Authority (CA) certificate.
    caCertPath: "/path/to/your/certs/tlsca/tlsca.your-org.com-cert.pem"

    # Your ID card: your user's signed certificate.
    clientCertPath: "/path/to/your/certs/users/Admin@your-org.com/msp/signcerts/cert.pem"

    # Your secret key: the one nobody else should have.
    clientKeyPath: "/path/to/your/certs/users/Admin@your-org.com/msp/keystore/the_key.sk"

log:
  # How chatty do we want the proxy to be?
  # 'debug': for when you've had three coffees and need to know EVERYTHING.
  # 'info': the usual for a quiet day.
  # 'warn': only for things that should worry you a little.
  # 'error': for when things get really ugly.
  level: "info"

  # Do you want the logs in "robot" mode (json) or "human" mode (text)?
  format: "text"
```

Don't like YAML files? Are you more of an `export MY_VARIABLE=...` kind of person? No problem! The proxy is listening. You can override anything in the `config.yaml` with environment variables.
