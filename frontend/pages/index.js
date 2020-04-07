import useSWR from "swr"

import fetch from "../libs/fetch"

function HomePage() {
  const { data, revalidate } = useSWR("/logins/", fetch)

  return data ? (
    <div>
      <button type='button' onClick={() => revalidate()}>
        Refresh
      </button>

      <span>{data.TotalData}</span>
    </div>
  ) : (
    <div>loading...</div>
  )
}

export default HomePage
