import { Table } from "antd"
import useSWR from "swr"

import fetch from "../libs/fetch"

import NewForm from "../components/new-form"
import Header from "../components/header"

function HomePage() {
  const [showNewModal, setNewModal] = React.useState(false)
  const { data: pass, isValidating, revalidate } = useSWR("/logins/", fetch)

  const onNewPass = () => {
    setNewModal(true)
  }

  const onNewOk = () => {
    setNewModal(false)
  }

  const onNewCancel = () => {
    setNewModal(false)
  }

  return (
    <div className="app">
      <Header
        loading={isValidating}
        revalidate={revalidate}
        onNewPass={onNewPass}
      />

      <div className="app-table">
        <Table loading={isValidating} dataSource={pass ? pass.Data : []} />
      </div>

      <NewForm
        visible={showNewModal}
        onNewOk={onNewOk}
        onNewCancel={onNewCancel}
      />

      <style jsx global>{`
        body {
          padding: 30px;
        }
        .ant-input-prefix {
          opacity: 0.5;
          margin-right: 6px;
        }
      `}</style>

      <style jsx>{`
        .app {
          max-width: 600px;
          margin-left: auto;
          margin-right: auto;
        }
        .app-table {
          margin-top: 20px;
        }
      `}</style>
    </div>
  )
}

export default HomePage
