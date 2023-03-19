#░█▀█░█░█░▀█▀░█▀█░░░░░█▀▀░█▀▀░█▀█░█▀▀░█▀▄░█▀█░▀█▀░█▀▀░█▀▄
#░█▀█░█░█░░█░░█░█░▄▄▄░█░█░█▀▀░█░█░█▀▀░█▀▄░█▀█░░█░░█▀▀░█░█
#░▀░▀░▀▀▀░░▀░░▀▀▀░░░░░▀▀▀░▀▀▀░▀░▀░▀▀▀░▀░▀░▀░▀░░▀░░▀▀▀░▀▀░
# DO NOT UPDATE: This file is managed by "build-configs"

FROM public.ecr.aws/docker/library/node:18 as build

RUN apt-get update \
    && apt-get install --yes \
    apt-transport-https \
    bash \
    ca-certificates \
    curl \
    gnupg \
    && curl -sLf --retry 3 --tlsv1.2 --proto "=https" 'https://packages.doppler.com/public/cli/gpg.DE2A7741A397C129.key' | apt-key add - \
    && echo "deb https://packages.doppler.com/public/cli/deb/debian any-version main" | tee /etc/apt/sources.list.d/doppler-cli.list \
    && apt-get update && apt-get install doppler \
    && curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to /usr/local/bin \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /usr/src/app

COPY package.json yarn.lock ./

RUN yarn install --frozen-lockfile

COPY .eslintignore .eslintrc.js .npmignore jest.config.js tsconfig.json ./

COPY index.ts index.ts
COPY justfile justfile
COPY lib/ lib/
{% for command in dockerfile.build_post_install -%}
{{ command | safe }}
{% endfor %}
RUN yarn build
