import * as React from "react"
import useSWR, { mutate } from "swr"

import fetch from "../libs/fetch"

import NewForm from "../components/new-form"
import Header from "../components/header"
import PassTable from "../components/table"

function HomePage() {
  const [showNewModal, setNewModal] = React.useState(false)
  const { data: pass, isValidating, revalidate } = useSWR("/logins/", fetch)

  const onModalClose = () => {
    setNewModal(false)
  }

  const onModalOpen = () => {
    setNewModal(true)
  }

  const onSubmit = async (values, actions) => {
    try {
      await fetch("/logins/", { method: "POST", body: JSON.stringify(values) })
      setNewModal(false)
      revalidate()
    } catch (e) {
      console.log(e)
    } finally {
      actions.setSubmitting(false)
    }
  }

  return (
    <div className="app">
      <Header
        loading={isValidating}
        onDataRefresh={revalidate}
        onModalOpen={onModalOpen}
      />

      <div className="app-table">
        <PassTable loading={isValidating} data={pass ? pass.Data : []} />
      </div>

      <NewForm
        visible={showNewModal}
        onClose={onModalClose}
        onSubmit={onSubmit}
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
          max-width: 700px;
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
