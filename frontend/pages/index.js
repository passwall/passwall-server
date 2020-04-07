import useSWR from "swr"
import { Button } from "antd"

import fetch from "../libs/fetch"

function HomePage() {
  const { data, revalidate } = useSWR("/logins/", fetch)

  return data ? (
    <div>
      <Button type='primary' onClick={() => revalidate()}>
        Refresh
      </Button>

      <span>{data.TotalData}</span>
    </div>
  ) : (
    <div>loading...</div>
  )
}

export default HomePage
