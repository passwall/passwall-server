import { Table } from "antd"

const columns = [
  {
    title: "URL",
    key: "URL",
    render: (text) => <a>{text}</a>,
  },
  {
    title: "Username",
    key: "Username",
  },
  {
    title: "Password",
    key: "Password",
  },
]

function Table({ loading, data }) {
  return (
    <Table
      size="middle"
      loading={loading}
      columns={columns}
      dataSource={data}
    />
  )
}

export default Table
