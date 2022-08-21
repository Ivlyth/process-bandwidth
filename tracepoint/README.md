# application source doc.

## openssl 1.1.*
### version
* define SSL3_VERSION                    0x0300
* define TLS1_VERSION                    0x0301
* define TLS1_1_VERSION                  0x0302
* define TLS1_2_VERSION                  0x0303
* define TLS1_3_VERSION                  0x0304
* define DTLS1_VERSION                   0xFEFF
* define DTLS1_2_VERSION                 0xFEFD
* define DTLS1_BAD_VER                   0x0100

```bc
//https://github.com/openssl/openssl/blob/3e8f70c30d84861fcd257a6e280dc49e104eb145/ssl/ssl_local.h#L1068
struct ssl_st {
    /*
     * protocol version (one of SSL2_VERSION, SSL3_VERSION, TLS1_VERSION,
     * DTLS1_VERSION)
     */
    int version;
    /* SSLv3 */
    const SSL_METHOD *method;
    /*
     * There are 2 BIO's even though they are normally both the same.  This
     * is so data can be read and written to different handlers
     */
    /* used by SSL_read */
    BIO *rbio;
    /* used by SSL_write */
    BIO *wbio;
    /* used during session-id reuse to concatenate messages */
    BIO *bbio;

    // ...
}

//https://github.com/openssl/openssl/blob/OpenSSL_1_1_1-stable/crypto/bio/bio_local.h
struct bio_st {
    const BIO_METHOD *method;
    /* bio, mode, argp, argi, argl, ret */
    BIO_callback_fn callback;
    BIO_callback_fn_ex callback_ex;
    char *cb_arg;               /* first argument for the callback */
    int init;
    int shutdown;
    int flags;                  /* extra storage */
    int retry_reason;
    int num;
    void *ptr;
    struct bio_st *next_bio;    /* used by filter BIOs */
    struct bio_st *prev_bio;    /* used by filter BIOs */
    CRYPTO_REF_COUNT references;
    uint64_t num_read;
    uint64_t num_write;
    CRYPTO_EX_DATA ex_data;
    CRYPTO_RWLOCK *lock;
};
```


## openssl  1.0.*
https://github.com/openssl/openssl/blob/OpenSSL_1_0_0-stable/ssl/ssl.h#L1093
```bc
struct ssl_st {
    /*
     * protocol version (one of SSL2_VERSION, SSL3_VERSION, TLS1_VERSION,
     * DTLS1_VERSION)
     */
    int version;
    /* SSL_ST_CONNECT or SSL_ST_ACCEPT */
    int type;
    /* SSLv3 */
    const SSL_METHOD *method;
    /*
     * There are 2 BIO's even though they are normally both the same.  This
     * is so data can be read and written to different handlers
     */
# ifndef OPENSSL_NO_BIO
    /* used by SSL_read */
    BIO *rbio;
    /* used by SSL_write */
    BIO *wbio;
    /* used during session-id reuse to concatenate messages */
    BIO *bbio;
# else
    /* used by SSL_read */
    char *rbio;
    /* used by SSL_write */
    char *wbio;
    char *bbio;
# endif
    /*
```


https://github.com/openssl/openssl/blob/OpenSSL_1_0_0-stable/crypto/bio/bio.h
```bc
struct bio_st {
    BIO_METHOD *method;
    /* bio, mode, argp, argi, argl, ret */
    long (*callback) (struct bio_st *, int, const char *, int, long, long);
    char *cb_arg;               /* first argument for the callback */
    int init;
    int shutdown;
    int flags;                  /* extra storage */
    int retry_reason;
    int num;
    void *ptr;
    struct bio_st *next_bio;    /* used by filter BIOs */
    struct bio_st *prev_bio;    /* used by filter BIOs */
    int references;
    unsigned long num_read;
    unsigned long num_write;
    CRYPTO_EX_DATA ex_data;
};
```
