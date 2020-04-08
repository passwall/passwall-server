import { Space, Button, Typography } from "antd"
import { SyncOutlined, PlusOutlined } from "@ant-design/icons"

const { Title } = Typography

function Header({ loading, onNewPass = () => {}, revalidate = () => {} }) {
  return (
    <header className="header">
      <Space>
        <Title style={{ marginBottom: 0 }} level={2}>
          GPass
        </Title>

        <Button
          shape="circle"
          loading={loading}
          icon={<SyncOutlined />}
          onClick={() => revalidate()}
        />
      </Space>

      <Button
        shape="round"
        type="primary"
        icon={<PlusOutlined />}
        onClick={onNewPass}
      >
        New Pass
      </Button>

      <style jsx>{`
        .header {
          display: grid;
          grid-template-columns: 1fr auto;
          align-items: center;
        }
      `}</style>
    </header>
  )
}

export default Header
