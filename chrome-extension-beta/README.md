# chrome-extension-form-autofill
Source code of a simple chrome extension to autofill form, which will help you get started down to your path as a chrome extension master

Learn to write your own simple chrome extension
In this article you’ll get to learn how easy it is to create a simple chrome extension(basic knowledge of HTML and Javascript is required) which will help you get started down to your path as a chrome extension master. At the end of the article you can find the github link to download the source code of the demo extension.

What exactly a Chrome Extension is?
Chrome extensions are a set of software codes that can modify and enhance the functionality of the Chrome browser.

It is very easy to write your first chrome extension. How??? Well it all exactly requires is a basic knowledge of HTML and Javascript. Knowledge of CSS can be an added advantage to make your extension more presentable but is not a necessary requirement.

So, how do we get started with it?
You’d require 4 important files, namely:
1. manifest.json
2. icon.png
3. popup.html
4. popup.js
Save all the 4 files in one single folder.

///Files explanation///

manifest.json
This is a JSON file, its like a metadata file which contains different properties. Some of the properties that is required to get your code started are mentioned below:
  1] manifest_version: Current value for this is “2”. Version 1 was deprecated in Chrome 18
  2] name: Name of your extension
  3] description: A plain text string which describes the browser user what your extension does.
  4] version: Version of your extension
  5] browser_action: It is used to put icon in the chrome toolbar, tooltip to that icon and a popup which will be opened once the icon is clicked.
  6] permissions: Permissions are required if your chrome extension wants to interact with the code running on browser pages. Following are different types of permission available:
   a] You can specify particular URL of the website for which the extension is created e.g.: “http://www.google.com/”
   b] “http://*/*” or https://*/* to match any URL that uses the http: or https: scheme
   c] “<all_urls>” to match any URL that starts with a permitted scheme like http:, https:, file:, ftp:, chrome-extension:
   d] “activeTab” gives an extension temporary access to the currently active tab
To check out other properties visit : https://developer.chrome.com/extensions/manifest 


icon.png
This image will be displayed next to the address bar. Extension will be executed once user click on this image. Any format is supported but preferred format is PNG since, PNG has the best support for transparency.


popup.html
This is a standard HTML file. This will give you control over what to display in a popup that will be opened once the user clicks on the above mentioned icon.


popup.js
The actual logic of what the extension is supposed to do is mentioned in this file. Out of all the methods, there are these 2 methods which are really helpful when it comes to developing a basic extension.
1] executeScript:
  Syntax: chrome.tabs.executeScript(tabId, details, callback function)
  tabId: ID of the browser tab where the script will be executed. “null” represents active tab of the current window, this is the default value.
  details: Enter the javascript code or the javascript file properties(both cannot be specified at once). One needs to be careful when using code parameter as it may open the extension to cross site scripting attacks.
callback function: This function is called when the above mentioned javascript code/file is executed.

2] insertCSS:
Syntax: chrome.tabs.insertCSS(tabId, details, callback function)
tabId: ID of the browser tab where the script will be executed. “null” represents active tab of the current window, this is the default value.
details: Enter the css code or the css file properties(both cannot be specified at once). One needs to be careful when using code parameter as it may open the extension to cross site scripting attacks.
callback function: This function is called when the above mentioned css code/file is executed.


There are many other methods which can be used when developing a full fledged extension https://developer.chrome.com/extensions/tabs/4
Communication with the database can be done through XMLHttpRequest().

Hooray!!! 
That’s all you need to know to start with your simple extension. Now your extension files are ready to be uploaded to chrome://extensions.

Steps to upload files in chrome://extensions:
Go to the URL: chrome://extensions in your google chrome browser
Check Developer Mode checkbox in the top right corner
Click on “Load unpacked extension” button
Navigate to your folder where the files reside

And that’s it, you can see your extension icon beside the address bar.

But what do I do if I have to make any code changes after uploading my files???
Steps to reflect code changes in the extension:
Make changes in your file and save it.
Go to the URL: chrome://extensions in your google chrome browser
Find your extension and click on Reload link.

Changes will be updated to the extensions without browser/tab refresh.

Debugging the extension:
Right click on the extension
Select Inspect Popup under the menu item and start debugging

So for everyone out there who wants to try out in coding an extension, I hope this article has been of a help to get you started.
