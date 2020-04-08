import * as React from "react"
import { Modal } from "antd"
import { Form, FormItem, Input, SubmitButton } from "formik-antd"
import { Formik } from "formik"
import { GlobalOutlined, UserOutlined, LockOutlined } from "@ant-design/icons"

function NewForm({ visible, onNewOk, onNewCancel }) {
  const formRef = React.useRef()

  const handleSubmit = () => {
    if (formRef.current) {
      console.log("submit form")
      //formRef.current.handleSubmit()
    }
  }

  return (
    <Modal
      title="Basic Modal"
      visible={visible}
      closable={false}
      maskClosable={false}
      onCancel={onNewCancel}
      onOk={handleSubmit}
    >
      <Formik
        innerRef={formRef}
        initialValues={{ URL: "", Username: "", Password: "" }}
        onSubmit={(values, actions) => {
          message.info(JSON.stringify(values, null, 4))
          actions.setSubmitting(false)
          actions.resetForm()
        }}
        validate={(values) => {
          if (!values.lastName) {
            return { lastName: "required" }
          }
          return {}
        }}
      >
        {() => (
          <Form layout="vertical">
            <FormItem label="URL" name="URL" required={true}>
              <Input
                name="URL"
                prefix={<GlobalOutlined />}
                placeholder="https://example.com"
              />
            </FormItem>

            <FormItem label="Username" name="Username" required={true}>
              <Input
                name="Username"
                prefix={<UserOutlined />}
                placeholder="Username or email"
              />
            </FormItem>

            <FormItem label="Password" name="Password" required={true}>
              <Input.Password
                name="Password"
                prefix={<LockOutlined />}
                placeholder="input placeholder"
              />
            </FormItem>
          </Form>
        )}
      </Formik>
    </Modal>
  )
}

export default NewForm
