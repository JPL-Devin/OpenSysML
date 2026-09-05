- **`python -m opensysml.generate` connects on its own.** The generator went through the
  module-level default connection, which is kept per address for the life of the process along
  with the service's handshake. Run twice in one process against services that took turns on one
  port, the second run was judged by the first service's handshake and could refuse a current
  service as too old. Each run now opens a connection of its own and closes it when done.
