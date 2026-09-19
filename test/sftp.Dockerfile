FROM alpine:latest

RUN apk add --no-cache openssh-server shadow \
    && ssh-keygen -A \
    && mkdir -p /var/run/sshd /home/testuser/debian \
    && adduser -D -s /bin/sh testuser \
    && echo "testuser:testpassword" | chpasswd \
    && chown -R testuser:testuser /home/testuser

RUN sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config \
    && sed -i 's/PasswordAuthentication no/PasswordAuthentication yes/' /etc/ssh/sshd_config \
    && echo "Subsystem sftp internal-sftp" >> /etc/ssh/sshd_config

EXPOSE 22

CMD ["/usr/sbin/sshd", "-D", "-e"]
