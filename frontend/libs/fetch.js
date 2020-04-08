import fetch from "isomorphic-unfetch"

const URL = "http://localhost:3625"

export default async function (path, options) {
  const res = await fetch(`${URL}${path}`, {
    headers: new Headers({
      Authorization: "Basic " + btoa("gpass" + ":" + "password"),
    }),
    ...options,
  })
  return res.json()
}
