#
#  vim:ts=2:sw=2:et
#

FROM quay.io/clearai/base:alpine-3.8
MAINTAINER Philip Stevenson <phil@clear.ai>

ADD ./bin/openvpn-authd /opt/bin/openvpn-authd
ADD ./templates/ /opt/bin/templates
RUN chmod +x /opt/bin/openvpn-authd
RUN mkdir /opt/bin/static
RUN apk add --no-cache --update ca-certificates
WORKDIR "/opt/bin"

ENTRYPOINT [ "/opt/bin/openvpn-authd" ]
