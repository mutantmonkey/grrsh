divert(-1)
define(`SERVER_ADDR', `localhost:31337')
define(`SERVER_LISTEN_ADDR', `:31337')
define(`DEFAULT_SHELL', `bash')
define(`ENABLE_RETRY', `true')
define(`CLIENT_PROXY_URL', `')
define(`CLIENT_PUBLIC_KEY', include(`keys/client/id_rsa.pub'))
define(`CLIENT_PRIVATE_KEY', include(`keys/client/id_rsa'))
define(`SERVER_PUBLIC_KEY', include(`keys/server/id_rsa.pub'))
define(`SERVER_PRIVATE_KEY', include(`keys/server/id_rsa'))
changequote(<,>)
divert(0)dnl
