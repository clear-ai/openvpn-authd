#
#  vim:ts=2:sw=2:et
#

FROM alpine:3.8
MAINTAINER Philip Stevenson <phil@clear.ai>

ADD ./bin/openvpn-authd /opt/bin/openvpn-authd
ADD templates/ /opt/bin/templates
RUN chmod +x /opt/bin/openvpn-authd

WORKDIR "/opt/bin"

ENTRYPOINT [ "/opt/bin/openvpn-authd" ]
