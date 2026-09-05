- **The Python client asks the service for uncompressed responses.** Under load, grpcio
  occasionally handed a gzip-compressed response to the protobuf parser, so an RPC the service
  had answered correctly failed with `Exception deserializing response!` or `Wire format was
  corrupt`. Every channel the client opens now accepts identity encoding only, which the service
  honours, so no response reaches the parser compressed.
