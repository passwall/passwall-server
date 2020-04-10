import * as React from "react"
import { Typography, Button, Space } from "antd"
import { LockOutlined } from "@ant-design/icons"

const { Paragraph } = Typography

function PasswordField({ children }) {
  const [show, setShow] = React.useState(false)

  return (
    <Space size={0}>
      <Paragraph style={{ marginBottom: 0 }} copyable>
        {show ? children : "• • • • • • • •"}
      </Paragraph>
      <Button type="link" onClick={() => setShow(!show)}>
        <LockOutlined />
      </Button>
    </Space>
  )
}

export default PasswordField
