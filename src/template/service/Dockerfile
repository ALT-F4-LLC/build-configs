#░█▀█░█░█░▀█▀░█▀█░░░░░█▀▀░█▀▀░█▀█░█▀▀░█▀▄░█▀█░▀█▀░█▀▀░█▀▄
#░█▀█░█░█░░█░░█░█░▄▄▄░█░█░█▀▀░█░█░█▀▀░█▀▄░█▀█░░█░░█▀▀░█░█
#░▀░▀░▀▀▀░░▀░░▀▀▀░░░░░▀▀▀░▀▀▀░▀░▀░▀▀▀░▀░▀░▀░▀░░▀░░▀▀▀░▀▀░
# DO NOT UPDATE: This file is managed by "build-configs"

FROM public.ecr.aws/docker/library/node:18-alpine3.16 as build

ARG NPM_TOKEN

{% if service.database %}
RUN yarn global add envsub@4.0.7
{% endif %}

WORKDIR /usr/src/app

COPY .npmrc package.json yarn.lock ./

RUN yarn install --frozen-lockfile \
    && rm -rf .npmrc

COPY src ./src
COPY docker-entrypoint.sh ./docker-entrypoint.sh
COPY tsconfig.json ./tsconfig.json
{% for command in dockerfile.build_post_install -%}
{{ command | safe }}
{% endfor %}

RUN yarn build

ENTRYPOINT [ "./docker-entrypoint.sh" ]


FROM public.ecr.aws/docker/library/node:18-alpine3.16 as service

ARG NPM_TOKEN

{% if service.database %}
RUN yarn global add envsub@4.0.7
{% endif %}

WORKDIR /usr/src/app

COPY .npmrc package.json yarn.lock ./

RUN yarn install --frozen-lockfile --production \
    && rm -rf .npmrc

COPY --from=build /usr/src/app/dist .
COPY docker-entrypoint.sh .
{% for command in dockerfile.service_post_install -%}
{{ command | safe }}
{% endfor %}

EXPOSE 9001

ENTRYPOINT [ "./docker-entrypoint.sh" ]
