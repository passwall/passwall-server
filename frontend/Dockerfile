# Install npm packages
FROM node:12-alpine as builder

WORKDIR /app

COPY package.json .

RUN yarn install --prod


# Push js files
FROM node:12-alpine

WORKDIR /app

COPY --from=builder /app/ /app/

COPY ./pages ./pages

COPY ./components ./components

COPY ./libs ./libs

RUN yarn build

CMD yarn start
