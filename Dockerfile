FROM registry.access.redhat.com/ubi8/ubi-minimal
ADD server /bin
CMD ["/bin/server"]