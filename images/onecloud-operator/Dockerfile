FROM alpine
RUN apk add tzdata --no-cache
ENV TZ Asia/Shanghai
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
ADD ./_output/bin/onecloud-controller-manager /bin/onecloud-controller-manager
