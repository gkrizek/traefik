#!/bin/bash

openssl req -x509 -newkey rsa:2048 -subj '/CN=voltageapp.io' -nodes -keyout default.key -out default.cert -days 730