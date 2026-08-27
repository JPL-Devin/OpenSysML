package io.opensysml;

/**
 * The service could not be reached, or the answer could not be read: a failure of the transport
 * rather than of the service.
 *
 * <p>Reported as {@link StatusCode#UNAVAILABLE}, which is what the Connect protocol maps a failed
 * connection to.
 */
public class TransportException extends ServiceException {

  private static final long serialVersionUID = 1L;

  /**
   * Creates a transport exception.
   *
   * @param message what failed
   * @param cause the underlying I/O failure
   */
  public TransportException(String message, Throwable cause) {
    super(StatusCode.UNAVAILABLE, message, cause);
  }
}
