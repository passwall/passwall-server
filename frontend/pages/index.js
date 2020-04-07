import useSWR from "swr"
import { Space, Table, Button, Typography } from "antd"
import fetch from "../libs/fetch"
import { SyncOutlined, PlusOutlined } from "@ant-design/icons"
import NewForm from "../components/new-form"

const { Title } = Typography

const columns = [
  {
    title: "Name",
    dataIndex: "name",
    key: "name",
    render: (text) => <a>{text}</a>,
  },
  {
    title: "Age",
    dataIndex: "age",
    key: "age",
  },
  {
    title: "Address",
    dataIndex: "address",
    key: "address",
  },
]

function HomePage() {
  const [showNewModal, setNewModal] = React.useState(false)
  const { data: pass, isValidating, revalidate } = useSWR("/logins/", fetch)

  const onNewOk = () => {
    setNewModal(false)
  }

  const onNewCancel = () => {
    setNewModal(false)
  }

  return (
    <div className='app'>
      <header className='app-header'>
        <Title level={2}>GPass</Title>
        <Space>
          <Button
            shape='round'
            type='primary'
            icon={<PlusOutlined />}
            onClick={() => setNewModal(true)}
          >
            New Pass
          </Button>
          <Button
            shape='round'
            loading={isValidating}
            icon={<SyncOutlined />}
            onClick={() => revalidate()}
          >
            Refresh
          </Button>
        </Space>
      </header>

      <div className='app-table'>
        <Table
          loading={isValidating}
          columns={columns}
          dataSource={pass ? pass.Data : []}
        />
      </div>

      <NewForm
        visible={showNewModal}
        onNewOk={onNewOk}
        onNewCancel={onNewCancel}
      />

      <style jsx>{`
        .app {
          padding: 30px;
        }
        .app-header {
        }
        .app-table {
          margin-top: 20px;
        }
      `}</style>
    </div>
  )
}

export default HomePage
