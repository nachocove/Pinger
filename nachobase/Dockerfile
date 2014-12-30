FROM golang
MAINTAINER Jan Vilhuber <janv@nachocove.com>
RUN apt-get update -qq -y && apt-get install -qq -y apt-utils supervisor
RUN addgroup nachocove
RUN adduser --ingroup nachocove --gecos "Nacho Cove User" --disabled-password nachocove
RUN update-rc.d supervisor defaults
