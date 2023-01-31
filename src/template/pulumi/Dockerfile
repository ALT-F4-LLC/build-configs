FROM public.ecr.aws/docker/library/node:18-alpine3.16 as build

ENV PATH="/root/.pulumi/bin:$PATH"

RUN wget -q -t3 'https://packages.doppler.com/public/cli/rsa.8004D9FF50437357.key' \
    -O /etc/apk/keys/cli@doppler-8004D9FF50437357.rsa.pub \
    && echo 'https://packages.doppler.com/public/cli/alpine/any-version/main' \
    | tee -a /etc/apk/repositories \
    && apk add --no-cache \
    curl \
    bash \
    doppler \
    && curl \
    --proto '=https' \
    --tlsv1.2 -sSf https://just.systems/install.sh \
    | bash -s -- --to /usr/local/bin \
    && curl -fsSL https://get.pulumi.com -o /root/setup_pulumi.sh \
    && chmod +x /root/setup_pulumi.sh \
    && /root/setup_pulumi.sh \
    && rm -rf /root/setup_pulumi.sh

WORKDIR /usr/src/app

COPY package.json yarn.lock ./

RUN yarn install --frozen-lockfile

COPY resource/ resource/
COPY index.ts index.ts
COPY justfile justfile
COPY Pulumi.yaml Pulumi.yaml
COPY tsconfig.json tsconfig.json
{% for command in dockerfile.build_post_install -%}
{{ command | safe }}
{% endfor %}

ENTRYPOINT ["pulumi"]
